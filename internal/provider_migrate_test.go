package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/wraient/curd/internal/loadproviders"
)

func TestProviderSelectionOptionsUsesDefaultAndSingleProviders(t *testing.T) {
	withAllProvidersEnabledForTest(t)
	options := providerSelectionOptions()
	if len(options) == 0 {
		t.Fatal("expected provider options")
	}
	if options[0].Key != "stacked" {
		t.Fatalf("first option key = %q, want stacked", options[0].Key)
	}
	if !strings.Contains(options[0].Label, "Default with fallback") {
		t.Fatalf("first option label = %q", options[0].Label)
	}
	for _, option := range options {
		if strings.Contains(option.Label, ", then ") {
			t.Fatalf("unexpected combo label %q", option.Label)
		}
	}
}

func TestMigrateProviderConfig(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    string
		changed bool
	}{
		{name: "legacy senshi default", raw: `["senshi"]`, want: "stacked", changed: true},
		{name: "legacy allanime default", raw: `["allanime"]`, want: "stacked", changed: true},
		{name: "stack alias", raw: "stack", want: "stacked", changed: true},
		{name: "already stacked", raw: "stacked", want: "stacked", changed: false},
		{name: "single anineko", raw: `["anineko"]`, want: `["anineko"]`, changed: false},
		{name: "legacy pair", raw: `["senshi","anineko"]`, want: "stacked", changed: true},
	}

	for _, tc := range cases {
		got, changed := migrateProviderConfig(tc.raw)
		if changed != tc.changed || got != tc.want {
			t.Fatalf("%s: got (%q, %v), want (%q, %v)", tc.name, got, changed, tc.want, tc.changed)
		}
	}
}

func TestCanonicalProviderConfigValuePrefersStackedToken(t *testing.T) {
	withAllProvidersEnabledForTest(t)
	if got := canonicalProviderConfigValue("stacked"); got != "stacked" {
		t.Fatalf("got %q, want stacked", got)
	}
	if got := canonicalProviderConfigValue(""); got != "stacked" {
		t.Fatalf("empty got %q, want stacked", got)
	}
}

func TestMigrateOnVersionUpgradeWritesVersionAndUpdatesProvider(t *testing.T) {
	withAllProvidersEnabledForTest(t)
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "curd.conf")
	storagePath := filepath.Join(tempDir, "share")
	if err := os.WriteFile(configPath, []byte("Provider=[\"senshi\"]\nStoragePath="+storagePath+"\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if config.Provider != `["senshi"]` {
		t.Fatalf("pre-migration provider = %q", config.Provider)
	}

	updated, err := MigrateOnVersionUpgrade(configPath, &config, "2.1.0")
	if err != nil {
		t.Fatalf("MigrateOnVersionUpgrade: %v", err)
	}
	if !updated {
		t.Fatal("expected provider config update")
	}
	if config.Provider != "stacked" {
		t.Fatalf("provider = %q, want stacked", config.Provider)
	}

	versionBytes, err := os.ReadFile(filepath.Join(storagePath, "curd_version"))
	if err != nil {
		t.Fatalf("read version file: %v", err)
	}
	if strings.TrimSpace(string(versionBytes)) != "2.1.0" {
		t.Fatalf("version file = %q", string(versionBytes))
	}

	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(contents), "Provider=stacked") {
		t.Fatalf("config not persisted: %s", string(contents))
	}

	updated, err = MigrateOnVersionUpgrade(configPath, &config, "2.1.0")
	if err != nil {
		t.Fatalf("second migration: %v", err)
	}
	if updated {
		t.Fatal("expected no update on same version")
	}
}

func TestLoadConfigAppendsMissingOptionsIncludingVimKeys(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "curd.conf")
	storagePath := filepath.Join(tempDir, "share")
	// Minimal config: no VimKeys — must be appended on load (not a full rewrite).
	initial := "StoragePath=" + storagePath + "\n" +
		"AddMissingOptions=true\n" +
		"Provider=stacked\n" +
		"Player=mpv\n"
	if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if config.VimKeys {
		t.Fatal("VimKeys default should be false")
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(after)
	if !strings.Contains(text, "VimKeys=false") {
		t.Fatalf("expected VimKeys=false in config file:\n%s", text)
	}
	// Original lines preserved (append-only).
	if !strings.Contains(text, "Player=mpv") || !strings.Contains(text, "StoragePath="+storagePath) {
		t.Fatalf("original keys should remain:\n%s", text)
	}

	// Second load must not duplicate VimKeys.
	if _, err := LoadConfig(configPath); err != nil {
		t.Fatalf("second LoadConfig: %v", err)
	}
	after2, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config again: %v", err)
	}
	if strings.Count(string(after2), "VimKeys=") != 1 {
		t.Fatalf("VimKeys should appear once, got:\n%s", after2)
	}
}

func TestMigrateOnVersionUpgradeInjectsMissingOptionsOnce(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "curd.conf")
	storagePath := filepath.Join(tempDir, "share")
	// Full-enough config written first so LoadConfig isn't under test here.
	initial := "StoragePath=" + storagePath + "\n" +
		"AddMissingOptions=true\n" +
		"Provider=stacked\n" +
		"Player=mpv\n"
	if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	// Pretend we're already on an older stored version with a sparse file.
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		t.Fatalf("mkdir storage: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storagePath, "curd_version"), []byte("2.0.0\n"), 0644); err != nil {
		t.Fatalf("write version: %v", err)
	}

	config := PopulateConfig(map[string]string{
		"StoragePath":       storagePath,
		"AddMissingOptions": "true",
		"Provider":          "stacked",
		"Player":            "mpv",
	})

	// Wipe keys that LoadConfig would have added so migration is the injector.
	if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
		t.Fatalf("rewrite sparse config: %v", err)
	}

	updated, err := MigrateOnVersionUpgrade(configPath, &config, "2.0.3")
	if err != nil {
		t.Fatalf("MigrateOnVersionUpgrade: %v", err)
	}
	if !updated {
		t.Fatal("expected config file to gain new options on version upgrade")
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after migrate: %v", err)
	}
	text := string(after)
	if !strings.Contains(text, "VimKeys=false") {
		t.Fatalf("expected VimKeys=false appended on upgrade:\n%s", text)
	}

	// Same version: no further rewrite.
	updated, err = MigrateOnVersionUpgrade(configPath, &config, "2.0.3")
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if updated {
		t.Fatal("same version must not re-inject options")
	}
}

func TestInjectMissingConfigDefaultsIdempotent(t *testing.T) {
	m := map[string]string{"Player": "mpv"}
	added := injectMissingConfigDefaults(m)
	if len(added) == 0 {
		t.Fatal("expected missing keys to be injected")
	}
	if _, ok := m["VimKeys"]; !ok {
		t.Fatal("expected VimKeys default")
	}
	if second := injectMissingConfigDefaults(m); len(second) != 0 {
		t.Fatalf("second inject should be empty, got %v", second)
	}
}

func TestConfiguredProviderNamesUsesStackedByDefault(t *testing.T) {
	withAllProvidersEnabledForTest(t)
	got := ConfiguredProviderNames(&CurdConfig{})
	want := []string{"senshi", "anipub", "anineko", "allanime", "animepahe"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
