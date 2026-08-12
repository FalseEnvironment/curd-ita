package animeworld

import (
	"encoding/json"
	"testing"

	"github.com/wraient/curd/internal/providers"
)

const samplePage = `
<html><body>
<h1 id="anime-title">Bocchi the Rock!</h1>
<span class="servers-tabs">
  <span class="server-tab" data-name="9">AnimeWorld Server</span>
  <span class="server-tab" data-name="4">Streamtape</span>
</span>
<div class="server active" data-name="9">
  <ul class="episodes">
    <li class="episode"><a href="/play/bocchi.AbC/ep1" data-episode-num="1" data-episode-id="101" data-id="9001">1</a></li>
    <li class="episode"><a href="/play/bocchi.AbC/ep2" data-episode-num="2" data-episode-id="102" data-id="9002">2</a></li>
    <li class="episode"><a href="/play/bocchi.AbC/ep3" data-episode-num="5.5" data-episode-id="103" data-id="9003">5.5</a></li>
  </ul>
</div>
<div class="server" data-name="4">
  <ul class="episodes">
    <li class="episode"><a href="/play/bocchi.AbC/ep1" data-episode-num="1" data-episode-id="101" data-id="4001">1</a></li>
  </ul>
</div>
</body></html>`

func TestParseEpisodesGroupsSourcesByServer(t *testing.T) {
	// Arrange / Act
	entries := parseEpisodes(samplePage)

	// Assert
	if len(entries) != 3 {
		t.Fatalf("expected 3 episodes, got %d", len(entries))
	}
	if entries[0].number != "1" || entries[1].number != "2" || entries[2].number != "5.5" {
		t.Fatalf("unexpected episode order: %+v", entries)
	}

	first := entries[0]
	if len(first.sources) != 2 {
		t.Fatalf("expected episode 1 on 2 servers, got %d", len(first.sources))
	}
	if first.sources[0].serverID != animeWorldServerID || first.sources[0].dataID != "9001" {
		t.Errorf("first source should come from the animeworld server block: %+v", first.sources[0])
	}
	if first.sources[1].serverID != 4 || first.sources[1].dataID != "4001" {
		t.Errorf("second source should come from server 4: %+v", first.sources[1])
	}
}

func TestParseEpisodesReturnsEmptyForStrippedPage(t *testing.T) {
	if entries := parseEpisodes("<html><body>Errore 404</body></html>"); len(entries) != 0 {
		t.Fatalf("expected no episodes, got %d", len(entries))
	}
}

func TestMatchEpisodeFallsBackToNumericValue(t *testing.T) {
	entries := []episodeEntry{{number: "01", numeric: 1}}

	entry, ok := matchEpisode(entries, 1)
	if !ok {
		t.Fatal("expected numeric fallback match")
	}
	if entry.number != "01" {
		t.Errorf("got %q, want %q", entry.number, "01")
	}

	if _, ok := matchEpisode(entries, 7); ok {
		t.Error("expected no match for missing episode")
	}
}

func TestPrioritizeSourcesPutsAnimeWorldServerFirst(t *testing.T) {
	sources := []episodeSource{
		{dataID: "a", serverID: 4},
		{dataID: "b", serverID: animeWorldServerID},
		{dataID: "c", serverID: 7},
	}

	ordered := prioritizeSources(sources)

	if ordered[0].dataID != "b" {
		t.Fatalf("expected animeworld server first, got %+v", ordered)
	}
	if ordered[1].dataID != "a" || ordered[2].dataID != "c" {
		t.Errorf("remaining order should be stable, got %+v", ordered)
	}
	if sources[0].dataID != "a" {
		t.Error("prioritizeSources must not mutate its input")
	}
}

func TestToSelectionOptionBuildsWatchKey(t *testing.T) {
	var entry map[string]json.RawMessage
	raw := `{
		"id": 42, "name": "Bocchi the Rock!", "jtitle": "Bocchi za Rokku!",
		"link": "bocchi-the-rock", "identifier": "AbC12",
		"episodes": "12", "dub": "0", "year": "2022", "studio": "CloverWorks",
		"image": "https://img.animeworld.ac/bocchi.jpg",
		"malId": 47917, "anilistId": 130003
	}`
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatalf("fixture is invalid: %v", err)
	}

	option, ok := toSelectionOption(entry)
	if !ok {
		t.Fatal("expected a usable option")
	}
	if option.Key != "bocchi-the-rock.AbC12" {
		t.Errorf("got key %q, want %q", option.Key, "bocchi-the-rock.AbC12")
	}

	item, ok := option.ExtraData.(SearchItem)
	if !ok {
		t.Fatal("ExtraData should carry a SearchItem")
	}
	if item.AnilistID != 130003 || item.MalID != 47917 {
		t.Errorf("tracker ids not parsed: %+v", item)
	}
	if item.Episodes != 12 {
		t.Errorf("got %d episodes, want 12", item.Episodes)
	}
	if item.Dub {
		t.Error(`dub "0" should parse as false`)
	}
}

func TestToSelectionOptionRejectsEntryWithoutWatchPath(t *testing.T) {
	var entry map[string]json.RawMessage
	if err := json.Unmarshal([]byte(`{"name": "No link", "identifier": "AbC"}`), &entry); err != nil {
		t.Fatalf("fixture is invalid: %v", err)
	}

	if _, ok := toSelectionOption(entry); ok {
		t.Error("expected entry without link to be skipped")
	}
}

func TestSortByModePrefersRequestedAudioWithoutDropping(t *testing.T) {
	options := []providers.SelectionOption{
		{Key: "sub", ExtraData: SearchItem{Dub: false}},
		{Key: "dub", ExtraData: SearchItem{Dub: true}},
	}

	sorted := sortByMode(options, "dub")

	if len(sorted) != 2 {
		t.Fatalf("expected both entries kept, got %d", len(sorted))
	}
	if sorted[0].Key != "dub" {
		t.Errorf("expected dub entry first, got %q", sorted[0].Key)
	}
}

func TestJSONHelpersHandleApiPlaceholders(t *testing.T) {
	var entry map[string]json.RawMessage
	raw := `{"a": "??", "b": null, "c": 7, "d": true, "e": "0", "f": 1}`
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		t.Fatalf("fixture is invalid: %v", err)
	}

	if got := jsonString(entry, "a"); got != "" {
		t.Errorf(`"??" should become empty, got %q`, got)
	}
	if got := jsonInt(entry, "b"); got != 0 {
		t.Errorf("null should become 0, got %d", got)
	}
	if got := jsonInt(entry, "c"); got != 7 {
		t.Errorf("got %d, want 7", got)
	}
	if got := jsonInt(entry, "missing"); got != 0 {
		t.Errorf("missing key should become 0, got %d", got)
	}
	if !jsonBool(entry, "d") || jsonBool(entry, "e") || !jsonBool(entry, "f") {
		t.Error("dub flag parsing is wrong for bool/string/number forms")
	}
}

func TestIsDirectMediaIgnoresQueryString(t *testing.T) {
	cases := map[string]bool{
		"https://server.animeworld.ac/video.mp4":           true,
		"https://server.animeworld.ac/video.mp4?token=abc": true,
		"https://server.animeworld.ac/hls/index.m3u8?t=1":  true,
		"https://streamtape.com/e/abcdef":                  false,
		"https://streamtape.com/e/abcdef?mp4=1":            false,
	}
	for link, want := range cases {
		if got := isDirectMedia(link); got != want {
			t.Errorf("isDirectMedia(%q) = %v, want %v", link, got, want)
		}
	}
}

func TestNumericValueParsesCompositeLabels(t *testing.T) {
	cases := map[string]float64{"1": 1, "5.5": 5.5, "268-269": 268, "OVA": 0}
	for label, want := range cases {
		if got := numericValue(label); got != want {
			t.Errorf("numericValue(%q) = %v, want %v", label, got, want)
		}
	}
}

func TestFindCookieAndCSRFToken(t *testing.T) {
	page := []byte(`<html><head>
	<meta name="csrf-token" id="csrf-token" content="tok3n">
	</head><body><script>document.cookie="AWCookieVerify=8f2a1c; path=/";</script></body></html>`)

	name, value, ok := findCookie(page)
	if !ok || name != "AWCookieVerify" || value != "8f2a1c" {
		t.Errorf("findCookie = (%q, %q, %v)", name, value, ok)
	}

	token, ok := findCSRFToken(page)
	if !ok || token != "tok3n" {
		t.Errorf("findCSRFToken = (%q, %v)", token, ok)
	}

	if _, _, ok := findCookie([]byte("<html></html>")); ok {
		t.Error("expected no cookie in a bare page")
	}
	if _, ok := findCSRFToken([]byte(`<meta name="viewport" content="width">`)); ok {
		t.Error("unrelated meta tags must not be treated as csrf tokens")
	}
}

func TestProviderNameAndRegistration(t *testing.T) {
	if name := (&Provider{}).Name(); name != "animeworld" {
		t.Errorf("got %q, want %q", name, "animeworld")
	}
	instance, err := providers.New("anime-world")
	if err != nil {
		t.Fatalf("alias lookup failed: %v", err)
	}
	if instance.Name() != "animeworld" {
		t.Errorf("alias resolved to %q", instance.Name())
	}
}
