package internal

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Placeholder media for non-current playlist rows. Short black frame so idle
// playlist construction never pulls real streams; on select we replace immediately.
const mpvEpisodePlaceholder = "av://lavfi:color=c=0x101010:s=16x16:d=1"

const (
	mpvPlaylistIdleDelay     = 3 * time.Second
	mpvPlaylistPollInterval  = 400 * time.Millisecond
	mpvPlaylistStableSamples = 3
)

// playlistSlot maps an MPV playlist index to curd episode + audio mode.
type playlistSlot struct {
	Episode int
	Mode    string // "sub" or "dub"
	Label   string
}

// MPVPlaylistController maintains a seamless MPV playlist of episodes (and
// optional alternate audio entry) without interrupting active playback while
// it is being built. Population happens only after playback is idle/stable.
type MPVPlaylistController struct {
	config *CurdConfig
	anime  *Anime
	socket string
	done   <-chan struct{}

	mu             sync.Mutex
	slots          []playlistSlot
	suppress       atomic.Bool // ignore playlist-pos churn we cause ourselves
	lastPos        int
	built          bool
	preferredMode  string
	hasAlternate   bool
	alternateMode  string
	currentPlaying int // episode number currently intended
	currentMode    string
}

// StartMPVPlaylistController waits until playback is stable, then builds the
// episode playlist and watches for user selections. Safe to call in a goroutine.
// Does nothing for android-intent / empty sockets or when disabled in config.
func StartMPVPlaylistController(config *CurdConfig, anime *Anime, socket string, done <-chan struct{}) {
	if config == nil || anime == nil || !config.MpvEpisodePlaylist {
		return
	}
	if socket == "" || socket == "android-intent" {
		return
	}

	c := &MPVPlaylistController{
		config:         config,
		anime:          anime,
		socket:         socket,
		done:           done,
		lastPos:        -1,
		preferredMode:  normalizeTranslationType(config.SubOrDub),
		currentPlaying: anime.Ep.Number,
		currentMode:    normalizeTranslationType(config.SubOrDub),
	}
	if c.preferredMode == "" {
		c.preferredMode = "sub"
	}
	c.currentMode = c.preferredMode
	c.alternateMode = alternateTranslationType(c.preferredMode)

	go c.run()
}

func (c *MPVPlaylistController) closed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *MPVPlaylistController) run() {
	if !c.waitUntilIdleStable() {
		return
	}
	if c.closed() {
		return
	}

	// Build episode rows without touching the currently playing demuxer path
	// beyond append/insert of placeholders.
	if err := c.buildEpisodePlaylist(); err != nil {
		Log(fmt.Sprintf("MPV playlist build failed: %v", err))
		// Still watch in case a partial list exists.
	} else {
		c.built = true
		Log("MPV episode playlist ready")
	}

	if c.closed() {
		return
	}

	// Probe alternate audio only after the list is up — never blocks first frame.
	c.probeAndAttachAlternateAudio()

	c.watchPlaylistSelection()
}

// waitUntilIdleStable waits until MPV reports a real time-pos a few times in a
// row so we do not mutate the playlist during initial buffering.
func (c *MPVPlaylistController) waitUntilIdleStable() bool {
	deadline := time.Now().Add(45 * time.Second)
	stable := 0
	// Initial grace so loadfile/subs settle.
	timer := time.NewTimer(mpvPlaylistIdleDelay)
	defer timer.Stop()
	select {
	case <-c.done:
		return false
	case <-timer.C:
	}

	for time.Now().Before(deadline) {
		if c.closed() {
			return false
		}
		pos, err := MPVSendCommand(c.socket, []interface{}{"get_property", "time-pos"})
		if err == nil && pos != nil {
			if mpvNumber(pos) >= 0 {
				stable++
				if stable >= mpvPlaylistStableSamples {
					return true
				}
				time.Sleep(mpvPlaylistPollInterval)
				continue
			}
		}
		stable = 0
		time.Sleep(mpvPlaylistPollInterval)
	}
	Log("MPV playlist: playback never became stable; skipping playlist build")
	return false
}

func (c *MPVPlaylistController) totalEpisodes() int {
	if c.anime.TotalEpisodes > 0 {
		return c.anime.TotalEpisodes
	}
	providerName, providerID := AnimeProviderID(c.anime)
	if providerID == "" {
		return 0
	}
	total, err := GetProviderTotalEpisodes(QualifyProviderID(providerName, providerID), c.preferredMode)
	if err != nil || total <= 0 {
		return 0
	}
	c.anime.TotalEpisodes = total
	return total
}

func (c *MPVPlaylistController) episodeLabel(ep int, mode string) string {
	name := GetAnimeName(*c.anime)
	base := fmt.Sprintf("Ep %02d", ep)
	if name != "" {
		base = fmt.Sprintf("%s · Ep %02d", name, ep)
	}
	if c.anime.FillerEpisodes != nil && IsEpisodeFiller(c.anime.FillerEpisodes, ep) {
		base += " [Filler]"
	}
	mode = normalizeTranslationType(mode)
	if mode != "" && mode != c.preferredMode {
		base += " · " + strings.ToUpper(mode)
	} else if mode == "dub" {
		base += " · DUB"
	}
	if ep == c.currentPlaying && mode == c.currentMode {
		base += "  ◀"
	}
	return base
}

func (c *MPVPlaylistController) buildEpisodePlaylist() error {
	total := c.totalEpisodes()
	if total <= 0 {
		return fmt.Errorf("unknown total episode count")
	}
	currentEp := c.anime.Ep.Number
	if currentEp < 1 {
		currentEp = 1
	}
	if currentEp > total {
		currentEp = total
	}

	c.suppress.Store(true)
	defer c.suppress.Store(false)

	// Title the currently playing item (index 0).
	_ = c.setPlaylistTitle(0, c.episodeLabel(currentEp, c.currentMode))

	// Insert previous episodes at the front (shifts current down).
	for ep := currentEp - 1; ep >= 1; ep-- {
		if c.closed() {
			return nil
		}
		if err := c.insertPlaceholderAt(0, ep, c.preferredMode); err != nil {
			Log(fmt.Sprintf("playlist insert ep %d: %v", ep, err))
		}
		// Tiny yield so IPC is not flooded mid-watch.
		time.Sleep(30 * time.Millisecond)
	}

	// Append future episodes.
	for ep := currentEp + 1; ep <= total; ep++ {
		if c.closed() {
			return nil
		}
		if err := c.appendPlaceholder(ep, c.preferredMode); err != nil {
			Log(fmt.Sprintf("playlist append ep %d: %v", ep, err))
		}
		time.Sleep(30 * time.Millisecond)
	}

	// Rebuild slot map: index i => episode i+1 (preferred mode).
	slots := make([]playlistSlot, 0, total)
	for ep := 1; ep <= total; ep++ {
		slots = append(slots, playlistSlot{
			Episode: ep,
			Mode:    c.preferredMode,
			Label:   c.episodeLabel(ep, c.preferredMode),
		})
	}
	c.mu.Lock()
	c.slots = slots
	c.currentPlaying = currentEp
	c.mu.Unlock()

	// Ensure MPV is still on the current episode index without forcing a reload
	// when already correct.
	wantPos := currentEp - 1
	if pos, err := c.playlistPos(); err == nil && pos != wantPos {
		// Setting playlist-pos would reload — only do it if we drifted.
		// After inserts, mpv usually keeps the playing entry selected.
		Log(fmt.Sprintf("MPV playlist-pos=%d want=%d (leaving as-is to avoid stutter)", pos, wantPos))
	}
	c.lastPos = wantPos
	if pos, err := c.playlistPos(); err == nil {
		c.lastPos = pos
	}
	return nil
}

func (c *MPVPlaylistController) insertPlaceholderAt(index, ep int, mode string) error {
	// loadfile <url> insert-at <index>
	_, err := MPVSendCommand(c.socket, []interface{}{"loadfile", mpvEpisodePlaceholder, "insert-at", index})
	if err != nil {
		// Older mpv: fall back to append (order less perfect but non-fatal).
		_, err = MPVSendCommand(c.socket, []interface{}{"loadfile", mpvEpisodePlaceholder, "append"})
		if err != nil {
			return err
		}
	}
	return c.setPlaylistTitle(index, c.episodeLabel(ep, mode))
}

func (c *MPVPlaylistController) appendPlaceholder(ep int, mode string) error {
	_, err := MPVSendCommand(c.socket, []interface{}{"loadfile", mpvEpisodePlaceholder, "append"})
	if err != nil {
		return err
	}
	count, err := c.playlistCount()
	if err != nil || count <= 0 {
		return err
	}
	return c.setPlaylistTitle(count-1, c.episodeLabel(ep, mode))
}

func (c *MPVPlaylistController) setPlaylistTitle(index int, title string) error {
	// mpv property: playlist/N/title
	_, err := MPVSendCommand(c.socket, []interface{}{"set_property", fmt.Sprintf("playlist/%d/title", index), title})
	return err
}

func (c *MPVPlaylistController) playlistPos() (int, error) {
	v, err := MPVSendCommand(c.socket, []interface{}{"get_property", "playlist-pos"})
	if err != nil {
		return -1, err
	}
	return int(mpvNumber(v) + 0.5), nil
}

func (c *MPVPlaylistController) playlistCount() (int, error) {
	v, err := MPVSendCommand(c.socket, []interface{}{"get_property", "playlist-count"})
	if err != nil {
		return 0, err
	}
	return int(mpvNumber(v) + 0.5), nil
}

// probeAndAttachAlternateAudio checks whether the opposite sub/dub mode has a
// stream for the *current* episode. If yes, inserts a playlist row so the user
// can switch audio by selecting it (full stream reload — only when chosen).
func (c *MPVPlaylistController) probeAndAttachAlternateAudio() {
	if c.closed() || c.anime == nil {
		return
	}
	alt := c.alternateMode
	cfg := *c.config
	cfg.SubOrDub = alt

	// Background resolve — does not touch MPV until we know a stream exists.
	result, err := ResolveEpisodeURL(cfg, c.anime, c.currentPlaying)
	if err != nil || len(result.Links) == 0 {
		Log(fmt.Sprintf("MPV playlist: no %s stream for ep %d (%v)", alt, c.currentPlaying, err))
		c.hasAlternate = false
		return
	}
	c.hasAlternate = true

	c.suppress.Store(true)
	defer c.suppress.Store(false)

	// Insert alternate entry right after the current episode index.
	c.mu.Lock()
	curIdx := c.currentPlaying - 1
	if curIdx < 0 {
		curIdx = 0
	}
	label := c.episodeLabel(c.currentPlaying, alt)
	// Physical insert in mpv
	insertAt := curIdx + 1
	c.mu.Unlock()

	if _, err := MPVSendCommand(c.socket, []interface{}{"loadfile", mpvEpisodePlaceholder, "insert-at", insertAt}); err != nil {
		// Fallback append
		if err := c.appendPlaceholder(c.currentPlaying, alt); err != nil {
			Log(fmt.Sprintf("MPV playlist: failed to add %s entry: %v", alt, err))
			return
		}
		c.mu.Lock()
		c.slots = append(c.slots, playlistSlot{Episode: c.currentPlaying, Mode: alt, Label: label})
		c.mu.Unlock()
		return
	}
	_ = c.setPlaylistTitle(insertAt, label)

	c.mu.Lock()
	// Insert into slots map at insertAt
	if insertAt >= len(c.slots) {
		c.slots = append(c.slots, playlistSlot{Episode: c.currentPlaying, Mode: alt, Label: label})
	} else {
		slot := playlistSlot{Episode: c.currentPlaying, Mode: alt, Label: label}
		c.slots = append(c.slots[:insertAt], append([]playlistSlot{slot}, c.slots[insertAt:]...)...)
	}
	c.mu.Unlock()
	Log(fmt.Sprintf("MPV playlist: added %s option for ep %d", alt, c.currentPlaying))
}

func (c *MPVPlaylistController) watchPlaylistSelection() {
	for {
		if c.closed() {
			return
		}
		if !IsMPVRunning(c.socket) {
			return
		}
		if c.suppress.Load() {
			time.Sleep(mpvPlaylistPollInterval)
			continue
		}
		pos, err := c.playlistPos()
		if err != nil {
			time.Sleep(mpvPlaylistPollInterval)
			continue
		}
		if pos < 0 {
			time.Sleep(mpvPlaylistPollInterval)
			continue
		}
		if pos == c.lastPos {
			time.Sleep(mpvPlaylistPollInterval)
			continue
		}
		// Debounce rapid scrubbing through the playlist UI.
		time.Sleep(200 * time.Millisecond)
		pos2, err2 := c.playlistPos()
		if err2 != nil || pos2 != pos {
			continue
		}
		c.lastPos = pos
		c.handlePlaylistJump(pos)
	}
}

func (c *MPVPlaylistController) handlePlaylistJump(pos int) {
	c.mu.Lock()
	if pos < 0 || pos >= len(c.slots) {
		c.mu.Unlock()
		return
	}
	slot := c.slots[pos]
	curEp, curMode := c.currentPlaying, c.currentMode
	c.mu.Unlock()

	if slot.Episode == curEp && normalizeTranslationType(slot.Mode) == normalizeTranslationType(curMode) {
		return
	}

	Log(fmt.Sprintf("MPV playlist: user selected ep %d (%s) at index %d", slot.Episode, slot.Mode, pos))
	if err := c.playSlot(slot); err != nil {
		Log(fmt.Sprintf("MPV playlist: failed to play ep %d: %v", slot.Episode, err))
		CurdOut(fmt.Sprintf("Could not play episode %d: %v", slot.Episode, err))
		// Try to restore previous playlist position without thrashing.
		c.suppress.Store(true)
		_, _ = MPVSendCommand(c.socket, []interface{}{"set_property", "playlist-pos", c.episodeIndex(curEp, curMode)})
		c.suppress.Store(false)
	}
}

func (c *MPVPlaylistController) episodeIndex(ep int, mode string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	mode = normalizeTranslationType(mode)
	for i, s := range c.slots {
		if s.Episode == ep && normalizeTranslationType(s.Mode) == mode {
			return i
		}
	}
	// Fallback: first slot with that episode number.
	for i, s := range c.slots {
		if s.Episode == ep {
			return i
		}
	}
	if ep > 0 {
		return ep - 1
	}
	return 0
}

func (c *MPVPlaylistController) playSlot(slot playlistSlot) error {
	c.suppress.Store(true)
	defer c.suppress.Store(false)

	mode := normalizeTranslationType(slot.Mode)
	if mode == "" {
		mode = c.preferredMode
	}

	cfg := *c.config
	cfg.SubOrDub = mode

	// Work on a shallow copy for provider id resolution side effects, then copy back.
	anime := c.anime
	prevEp := anime.Ep.Number
	anime.Ep.Number = slot.Episode

	result, err := ResolveEpisodeURLForPlayback(cfg, anime, slot.Episode)
	if err != nil || len(result.Links) == 0 {
		anime.Ep.Number = prevEp
		if err == nil {
			err = fmt.Errorf("no streams")
		}
		return err
	}

	anime.Ep.Links = result.Links
	applyStreamPlaybackHints(anime, result.Links, result.LinkHints)
	link := PrioritizeLink(result.Links)
	if link == "" {
		anime.Ep.Number = prevEp
		return fmt.Errorf("empty stream link")
	}

	title := c.episodeLabel(slot.Episode, mode)
	// Reuse running MPV instance — loadfile replace is the intentional user switch.
	anime.Ep.Player.SocketPath = c.socket
	if _, err := StartVideo(link, []string{}, title, anime); err != nil {
		anime.Ep.Number = prevEp
		return err
	}

	anime.Ep.Number = slot.Episode
	anime.Ep.Started = false
	anime.Ep.IsCompleted = false
	anime.Ep.Player.PlaybackTime = 0
	anime.Ep.Duration = 0
	anime.Ep.SkipTimes = SkipTimes{}

	c.mu.Lock()
	c.currentPlaying = slot.Episode
	c.currentMode = mode
	c.mu.Unlock()

	// Refresh skip times in background (non-blocking for playback).
	go func(ep int) {
		if c.anime.MalId > 0 {
			if err := GetAndParseAniSkipData(c.anime.MalId, ep, 0, c.anime); err != nil {
				Log(fmt.Sprintf("AniSkip for ep %d: %v", ep, err))
			}
			if c.anime.Ep.SkipTimes.Op.Start != c.anime.Ep.SkipTimes.Op.End ||
				c.anime.Ep.SkipTimes.Ed.Start != c.anime.Ep.SkipTimes.Ed.End {
				_ = SendSkipTimesToMPV(c.anime)
			}
		}
	}(slot.Episode)

	// Rebuild playlist markers after a short idle (titles ◀ indicator, alternate audio).
	go func() {
		time.Sleep(mpvPlaylistIdleDelay)
		if c.closed() {
			return
		}
		// Soft retitle only — avoid full playlist rebuild (would stutter).
		c.retitleSlots()
		// Re-probe alternate for the new episode.
		c.hasAlternate = false
		c.probeAndAttachAlternateAudio()
	}()

	CurdOut(fmt.Sprintf("Playing episode %d (%s)", slot.Episode, mode))
	return nil
}

func (c *MPVPlaylistController) retitleSlots() {
	c.suppress.Store(true)
	defer c.suppress.Store(false)
	c.mu.Lock()
	slots := append([]playlistSlot(nil), c.slots...)
	curEp, curMode := c.currentPlaying, c.currentMode
	c.mu.Unlock()
	for i, s := range slots {
		label := c.episodeLabel(s.Episode, s.Mode)
		// episodeLabel uses c.currentPlaying for ◀ marker
		_ = curEp
		_ = curMode
		_ = c.setPlaylistTitle(i, label)
		time.Sleep(15 * time.Millisecond)
	}
}
