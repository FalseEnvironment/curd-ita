package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

const (
	defaultUpdateRepo          = "Wraient/curd"
	updatePendingFileName      = "update_pending.json"
	backgroundUpdateIdleDelay  = 4 * time.Second
	defaultRemindLaterDuration = 24 * time.Hour
	maxReleaseNotesRunes       = 1200
)

// updatePendingState is persisted under StoragePath so startup never blocks on
// network — a previous idle check stores availability for the next launch.
type updatePendingState struct {
	Available      bool   `json:"available"`
	LatestVersion  string `json:"latest_version"`
	LatestTag      string `json:"latest_tag"`
	ReleaseName    string `json:"release_name"`
	ReleaseNotes   string `json:"release_notes"`
	HTMLURL        string `json:"html_url"`
	AssetName      string `json:"asset_name"`
	CheckedAt      string `json:"checked_at"`
	SkippedVersion string `json:"skipped_version,omitempty"`
	RemindAfter    string `json:"remind_after,omitempty"`
}

type githubReleaseAPI struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

var backgroundUpdateOnce sync.Once

func updatePendingPath(storagePath string) string {
	if strings.TrimSpace(storagePath) == "" {
		storagePath = GetStoragePath()
	}
	return filepath.Join(os.ExpandEnv(storagePath), updatePendingFileName)
}

func loadUpdatePendingState(storagePath string) updatePendingState {
	path := updatePendingPath(storagePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return updatePendingState{}
	}
	var state updatePendingState
	if err := json.Unmarshal(data, &state); err != nil {
		Log(fmt.Sprintf("Ignoring corrupt update state: %v", err))
		return updatePendingState{}
	}
	return state
}

func saveUpdatePendingState(storagePath string, state updatePendingState) error {
	path := updatePendingPath(storagePath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func curdReleaseBinaryName() (string, error) {
	switch runtime.GOOS {
	case "windows":
		if runtime.GOARCH == "arm64" {
			return "curd-windows-arm64.exe", nil
		}
		return "curd-windows-x86_64.exe", nil
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			return "curd-macos-x86_64", nil
		case "arm64":
			return "curd-macos-arm64", nil
		default:
			return "curd-macos-universal", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "curd-linux-x86_64", nil
		case "arm64":
			return "curd-linux-arm64", nil
		default:
			return "", fmt.Errorf("unsupported Linux architecture: %s", runtime.GOARCH)
		}
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func normalizeReleaseVersion(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "v")
	tag = strings.TrimPrefix(tag, "V")
	return tag
}

func truncateReleaseNotes(notes string) string {
	notes = strings.TrimSpace(notes)
	if notes == "" {
		return "(No release notes provided.)"
	}
	runes := []rune(notes)
	if len(runes) <= maxReleaseNotesRunes {
		return notes
	}
	return string(runes[:maxReleaseNotesRunes]) + "\n… (truncated)"
}

func fetchLatestGitHubRelease(repo string) (githubReleaseAPI, error) {
	if strings.TrimSpace(repo) == "" {
		repo = defaultUpdateRepo
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return githubReleaseAPI{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "curd-update-check")

	client := sharedHTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return githubReleaseAPI{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return githubReleaseAPI{}, fmt.Errorf("github releases API status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var release githubReleaseAPI
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return githubReleaseAPI{}, err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return githubReleaseAPI{}, fmt.Errorf("latest release has no tag")
	}
	return release, nil
}

func isUpdateNewer(latest, current string) bool {
	latest = normalizeReleaseVersion(latest)
	current = normalizeReleaseVersion(current)
	if latest == "" || current == "" {
		return false
	}
	return versionLess(current, latest)
}

// StartBackgroundUpdateCheck runs after a short idle delay so startup is not blocked.
// Results are written to StoragePath/update_pending.json for the next launch.
func StartBackgroundUpdateCheck(config *CurdConfig, currentVersion string) {
	if config == nil || !config.CheckUpdates {
		return
	}
	backgroundUpdateOnce.Do(func() {
		go func() {
			time.Sleep(backgroundUpdateIdleDelay)
			if err := checkForUpdateInBackground(config, currentVersion); err != nil {
				Log(fmt.Sprintf("Background update check failed: %v", err))
			}
		}()
	})
}

func checkForUpdateInBackground(config *CurdConfig, currentVersion string) error {
	storagePath := config.StoragePath
	state := loadUpdatePendingState(storagePath)

	// Honor "remind later" without hitting the network repeatedly if still waiting.
	if state.RemindAfter != "" {
		if until, err := time.Parse(time.RFC3339, state.RemindAfter); err == nil && time.Now().Before(until) {
			Log(fmt.Sprintf("Skipping update check until %s", state.RemindAfter))
			return nil
		}
	}

	release, err := fetchLatestGitHubRelease(defaultUpdateRepo)
	if err != nil {
		return err
	}
	latest := normalizeReleaseVersion(release.TagName)
	state.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	state.LatestTag = release.TagName
	state.LatestVersion = latest
	state.ReleaseName = strings.TrimSpace(release.Name)
	if state.ReleaseName == "" {
		state.ReleaseName = "Curd " + latest
	}
	state.ReleaseNotes = truncateReleaseNotes(release.Body)
	state.HTMLURL = release.HTMLURL

	if asset, assetErr := curdReleaseBinaryName(); assetErr == nil {
		state.AssetName = asset
	}

	if state.SkippedVersion != "" && normalizeReleaseVersion(state.SkippedVersion) == latest {
		state.Available = false
		return saveUpdatePendingState(storagePath, state)
	}

	if !isUpdateNewer(latest, currentVersion) {
		state.Available = false
		return saveUpdatePendingState(storagePath, state)
	}

	state.Available = true
	// Clear remind-later once a check found something actionable after the window.
	if state.RemindAfter != "" {
		if until, err := time.Parse(time.RFC3339, state.RemindAfter); err == nil && !time.Now().Before(until) {
			state.RemindAfter = ""
		}
	}
	Log(fmt.Sprintf("Update available: %s → %s", currentVersion, latest))
	return saveUpdatePendingState(storagePath, state)
}

func pendingUpdateShouldPrompt(config *CurdConfig, currentVersion string, state updatePendingState) bool {
	if config == nil || !config.CheckUpdates {
		return false
	}
	if !state.Available || strings.TrimSpace(state.LatestVersion) == "" {
		return false
	}
	if !isUpdateNewer(state.LatestVersion, currentVersion) {
		return false
	}
	if normalizeReleaseVersion(state.SkippedVersion) == normalizeReleaseVersion(state.LatestVersion) {
		return false
	}
	if state.RemindAfter != "" {
		if until, err := time.Parse(time.RFC3339, state.RemindAfter); err == nil && time.Now().Before(until) {
			return false
		}
	}
	return true
}

func formatLocalTime(t time.Time) string {
	return t.In(time.Local).Format("Mon Jan 2 2006, 3:04 PM MST")
}

func buildUpdatePromptMessage(currentVersion string, state updatePendingState) (prompt, message string) {
	from := normalizeReleaseVersion(currentVersion)
	to := normalizeReleaseVersion(state.LatestVersion)
	prompt = fmt.Sprintf("Update %s → %s", from, to)

	var b strings.Builder
	if state.ReleaseName != "" {
		fmt.Fprintf(&b, "%s\n", state.ReleaseName)
	}
	fmt.Fprintf(&b, "Current: %s   Latest: %s\n", from, to)
	if state.HTMLURL != "" {
		fmt.Fprintf(&b, "%s\n", state.HTMLURL)
	}
	b.WriteString("\n")
	b.WriteString(state.ReleaseNotes)
	message = strings.TrimSpace(b.String())
	// Rofi -mesg stays readable; keep a hard cap.
	if runes := []rune(message); len(runes) > maxReleaseNotesRunes {
		message = string(runes[:maxReleaseNotesRunes]) + "\n… (truncated)"
	}
	return prompt, message
}

// updateUserMessage prints a single status line. With Rofi mode, CurdOut becomes
// notify-send — so we only send one short notification (or log) for status.
func updateUserMessage(config *CurdConfig, msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	Log(msg)
	if config != nil && config.RofiSelection {
		// One short desktop notification, not a barrage of CurdOut lines.
		_ = exec.Command("notify-send", "-a", "Curd",
			"-h", "string:x-canonical-private-synchronous:curd-update",
			"Curd", msg).Run()
		return
	}
	fmt.Println(msg)
}

// HandlePendingUpdatePrompt shows a previously detected update (from idle check).
// Returns true if the caller should exit (user updated or chose to quit the session).
func HandlePendingUpdatePrompt(config *CurdConfig, currentVersion string) bool {
	if config == nil || !config.CheckUpdates {
		return false
	}
	state := loadUpdatePendingState(config.StoragePath)
	if !pendingUpdateShouldPrompt(config, currentVersion, state) {
		return false
	}

	// Numbered labels keep priority order under DynamicSelect's alphabetical sort.
	options := []SelectionOption{
		{Key: "update", Label: "1. Update now"},
		{Key: "later", Label: "2. Remind me later"},
		{Key: "skip", Label: "3. Skip this version"},
		{Key: "disable", Label: "4. Turn off automatic update checks"},
		{Key: "continue", Label: "5. Continue without updating"},
	}
	prompt, message := buildUpdatePromptMessage(currentVersion, state)

	var selected SelectionOption
	var err error
	if config.RofiSelection {
		// All details go in Rofi -mesg; zero notify-send spam for notes.
		selected, err = RofiSelectWithMessage(options, false, prompt, message)
	} else {
		fmt.Println(prompt)
		fmt.Println(message)
		fmt.Println()
		selected, err = promptSelect(options)
	}
	if err != nil || selected.Key == "-1" || selected.Key == "-2" || selected.Key == "continue" || selected.Key == "" {
		return false
	}

	switch selected.Key {
	case "update":
		updateUserMessage(config, "Downloading and installing update…")
		if err := UpdateCurd(defaultUpdateRepo, "curd"); err != nil {
			updateUserMessage(config, fmt.Sprintf("Update failed: %v", err))
			Log(fmt.Sprintf("Update failed: %v", err))
			return false
		}
		state.Available = false
		state.RemindAfter = ""
		_ = saveUpdatePendingState(config.StoragePath, state)
		updateUserMessage(config, fmt.Sprintf("Updated to %s. Please restart curd.", state.LatestVersion))
		return true
	case "later":
		until := time.Now().Add(defaultRemindLaterDuration)
		state.RemindAfter = until.UTC().Format(time.RFC3339)
		state.Available = true
		_ = saveUpdatePendingState(config.StoragePath, state)
		updateUserMessage(config, fmt.Sprintf("Will remind again after %s.", formatLocalTime(until)))
		return false
	case "skip":
		state.SkippedVersion = state.LatestVersion
		state.Available = false
		state.RemindAfter = ""
		_ = saveUpdatePendingState(config.StoragePath, state)
		updateUserMessage(config, fmt.Sprintf("Skipping version %s.", state.LatestVersion))
		return false
	case "disable":
		if err := setConfigBoolOption(GlobalConfigPath, "CheckUpdates", false); err != nil {
			updateUserMessage(config, fmt.Sprintf("Could not write config: %v", err))
			Log(fmt.Sprintf("disable CheckUpdates: %v", err))
		} else {
			config.CheckUpdates = false
			updateUserMessage(config, "Automatic update checks disabled (CheckUpdates=false).")
		}
		state.Available = false
		_ = saveUpdatePendingState(config.StoragePath, state)
		return false
	default:
		return false
	}
}

func setConfigBoolOption(configPath, key string, value bool) error {
	if strings.TrimSpace(configPath) == "" {
		return fmt.Errorf("config path is empty")
	}
	configMap, err := LoadConfigFromFile(configPath)
	if err != nil {
		return err
	}
	_, existed := configMap[key]
	if value {
		configMap[key] = "true"
	} else {
		configMap[key] = "false"
	}
	if !existed {
		return appendConfigKeys(configPath, configMap, []string{key})
	}
	return SaveConfigToFile(configPath, configMap)
}

func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "operation not permitted")
}

// isCrossDeviceError reports rename/link failures when src and dest are on
// different filesystems (e.g. /tmp tmpfs → ~/.local/bin on disk).
func isCrossDeviceError(err error) bool {
	if err == nil {
		return false
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		err = linkErr.Err
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cross-device") ||
		strings.Contains(msg, "cross device") ||
		strings.Contains(msg, "exdev") ||
		strings.Contains(msg, "invalid cross-device link")
}

func preferGUIPasswordPrompt() bool {
	if cfg := GetGlobalConfig(); cfg != nil && cfg.RofiSelection {
		return true
	}
	// No usable TTY (common when launched from a desktop entry / rofi).
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return true
	}
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

// promptSudoPasswordGUI tries GTK/desktop password dialogs (zenity → yad → kdialog).
func promptSudoPasswordGUI(prompt string) (string, error) {
	if prompt == "" {
		prompt = "Enter your password to install the Curd update:"
	}

	type dialog struct {
		name string
		args []string
	}
	dialogs := []dialog{
		// GTK (GNOME / many desktops)
		{"zenity", []string{"--password", "--title=Curd Update", "--text=" + prompt}},
		// GTK-based yad
		{"yad", []string{"--entry", "--hide-text", "--title=Curd Update", "--text=" + prompt, "--button=OK:0", "--button=Cancel:1"}},
		// KDE
		{"kdialog", []string{"--title", "Curd Update", "--password", prompt}},
	}

	for _, d := range dialogs {
		bin, err := exec.LookPath(d.name)
		if err != nil {
			continue
		}
		cmd := exec.Command(bin, d.args...)
		out, err := cmd.Output()
		if err != nil {
			// User cancel or dialog error — try next tool only on "not found"; cancel stops.
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				// zenity/yad/kdialog: non-zero usually means cancel.
				return "", fmt.Errorf("password dialog cancelled")
			}
			Log(fmt.Sprintf("password dialog %s failed: %v", d.name, err))
			continue
		}
		password := strings.TrimRight(string(out), "\r\n")
		if password == "" {
			return "", fmt.Errorf("empty password")
		}
		Log(fmt.Sprintf("Collected sudo password via %s dialog", d.name))
		return password, nil
	}
	return "", fmt.Errorf("no GUI password dialog available (tried zenity, yad, kdialog)")
}

func promptSudoPassword(prompt string) (string, error) {
	if prompt == "" {
		prompt = "Administrator password (sudo) to install update: "
	}

	// Rofi / desktop: prefer GTK password dialog so the user isn't dumped to a TTY.
	if preferGUIPasswordPrompt() {
		if password, err := promptSudoPasswordGUI(prompt); err == nil {
			return password, nil
		} else {
			Log(fmt.Sprintf("GUI password prompt unavailable (%v); falling back to terminal", err))
			// If cancel was explicit, don't fall through to a broken TTY prompt in rofi mode.
			if strings.Contains(err.Error(), "cancelled") {
				return "", err
			}
		}
	}

	// Terminal fallback (CLI mode or no GUI tool installed).
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("no terminal available for password entry; install zenity for a GUI prompt")
	}
	fmt.Fprint(os.Stderr, prompt)
	if !strings.HasSuffix(prompt, " ") {
		fmt.Fprint(os.Stderr, " ")
	}
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(password), nil
}

func installExecutableWithSudo(src, dest string) error {
	// Prefer install(1) for mode bits; fall back to cp + chmod.
	password, err := promptSudoPassword("Administrator password (sudo) to install update: ")
	if err != nil {
		return fmt.Errorf("read sudo password: %w", err)
	}
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("empty sudo password")
	}

	try := func(name string, args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Stdin = strings.NewReader(password + "\n")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// -S: read password from stdin; -p '': no extra prompt
	if err := try("sudo", "-S", "-p", "", "install", "-m", "755", src, dest); err == nil {
		return nil
	} else {
		Log(fmt.Sprintf("sudo install failed: %v", err))
	}
	if err := try("sudo", "-S", "-p", "", "cp", src, dest); err != nil {
		return fmt.Errorf("sudo install failed: %w", err)
	}
	_ = try("sudo", "-S", "-p", "", "chmod", "755", dest)
	return nil
}

// createUpdateTempFile prefers a sibling of the executable (same FS as install
// path). Falls back to os.TempDir when the install directory is not writable.
func createUpdateTempFile(executablePath, binaryName string) (string, *os.File, error) {
	dir := filepath.Dir(executablePath)
	// Try same directory first for atomic rename.
	if f, err := os.CreateTemp(dir, ".curd-download-*"); err == nil {
		return f.Name(), f, nil
	}
	// Fall back to system temp (may be cross-device; replaceExecutable handles that).
	name := "curd-update-" + binaryName
	if binaryName == "" {
		name = "curd-update-bin"
	}
	path := filepath.Join(os.TempDir(), name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", nil, err
	}
	return path, f, nil
}

func copyFileReplace(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Write to a sibling temp on the destination filesystem, then rename into place.
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".curd-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(0755); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// Atomic replace when possible (same directory = same filesystem).
	if err := os.Rename(tmpName, dst); err != nil {
		// Windows / busy binary: move dest aside first.
		oldPath := dst + ".old"
		_ = os.Remove(oldPath)
		if renOld := os.Rename(dst, oldPath); renOld != nil && !os.IsNotExist(renOld) {
			return renOld
		}
		if renNew := os.Rename(tmpName, dst); renNew != nil {
			_ = os.Rename(oldPath, dst)
			return renNew
		}
		_ = os.Remove(oldPath)
	}
	cleanup = false
	return nil
}

// replaceExecutable installs the downloaded binary over the running executable.
// Handles: same-FS rename, cross-device copy, busy binary swap, and sudo when needed.
func replaceExecutable(tmpPath, executablePath string) error {
	// 1) Fast path: rename when both paths share a filesystem.
	if err := os.Rename(tmpPath, executablePath); err == nil {
		return nil
	} else if isPermissionError(err) {
		updateUserMessage(GetGlobalConfig(), "Update needs elevated permissions to replace the installed binary.")
		if sudoErr := installExecutableWithSudo(tmpPath, executablePath); sudoErr != nil {
			return sudoErr
		}
		_ = os.Remove(tmpPath)
		return nil
	} else if !isCrossDeviceError(err) {
		// Unexpected rename failure — still try copy-into-place before giving up.
		Log(fmt.Sprintf("rename to %s failed (%v); trying copy replace", executablePath, err))
	}

	// 2) Cross-device (or other rename failure): copy onto dest filesystem then swap.
	if err := copyFileReplace(tmpPath, executablePath); err == nil {
		_ = os.Remove(tmpPath)
		return nil
	} else if isPermissionError(err) {
		updateUserMessage(GetGlobalConfig(), "Update needs elevated permissions to replace the installed binary.")
		if sudoErr := installExecutableWithSudo(tmpPath, executablePath); sudoErr != nil {
			return fmt.Errorf("failed to replace executable: %w", sudoErr)
		}
		_ = os.Remove(tmpPath)
		return nil
	} else {
		// Last resort: sudo install from original temp (covers weird FS layouts).
		if isCrossDeviceError(err) || runtime.GOOS != "windows" {
			Log(fmt.Sprintf("copy replace failed (%v); trying sudo", err))
			updateUserMessage(GetGlobalConfig(), "Update needs elevated permissions to replace the installed binary.")
			if sudoErr := installExecutableWithSudo(tmpPath, executablePath); sudoErr == nil {
				_ = os.Remove(tmpPath)
				return nil
			}
		}
		return fmt.Errorf("failed to replace executable: %w", err)
	}
}
