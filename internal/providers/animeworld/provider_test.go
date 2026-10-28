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

func TestFilterByModeReturnsOnlyMatchingAudio(t *testing.T) {
	options := []providers.SelectionOption{
		{Key: "sub", ExtraData: SearchItem{Dub: false}},
		{Key: "dub", ExtraData: SearchItem{Dub: true}},
	}

	// Request dub - should only return dub entry
	filtered := filterByMode(options, "dub")

	if len(filtered) != 1 {
		t.Fatalf("expected only dub entry, got %d", len(filtered))
	}
	if filtered[0].Key != "dub" {
		t.Errorf("expected dub entry, got %q", filtered[0].Key)
	}

	// Request sub - should only return sub entry
	filtered = filterByMode(options, "sub")

	if len(filtered) != 1 {
		t.Fatalf("expected only sub entry, got %d", len(filtered))
	}
	if filtered[0].Key != "sub" {
		t.Errorf("expected sub entry, got %q", filtered[0].Key)
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

func TestParseShowTitleDetectsDubSuffix(t *testing.T) {
	dub := `<html><body><h1 id="anime-title">Ergo Proxy (ITA)</h1></body></html>`
	if got := parseShowTitle(dub); got != "Ergo Proxy (ITA)" {
		t.Fatalf("got %q", got)
	}
	if !isDubTitle(parseShowTitle(dub)) {
		t.Error("dub show title should be detected as dub")
	}

	sub := `<html><body><h1 id="anime-title">Ergo Proxy</h1></body></html>`
	if got := parseShowTitle(sub); got != "Ergo Proxy" {
		t.Fatalf("got %q", got)
	}
	if isDubTitle(parseShowTitle(sub)) {
		t.Error("sub show title must not be detected as dub")
	}

	if got := parseShowTitle("<html></html>"); got != "" {
		t.Errorf("missing title should be empty, got %q", got)
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

const sampleSearchPage = `
<html><body>
<div class="content">
  <div class="film-list">
    <div class="item">
      <div class="inner">
        <a href="/play/fairies-albums.71rpK" class="poster" data-tip="api/tooltip/82">
          <img loading="lazy" src="https://img.animeworld.ac/locandine/71rpK.png" alt="Fairies Albums" />
          <div class="status"><div class="ona">ONA</div><span class="bar"></span></div>
        </a>
        <a href="/play/fairies-albums.71rpK" data-jtitle="Bai Yao Pu" class="name">Fairies Albums</a>
      </div>
    </div>
    <div class="item">
      <div class="inner">
        <a href="/play/air.XDBaJ" class="poster" data-tip="api/tooltip/2070">
          <img loading="lazy" src="https://img.animeworld.ac/locandine/22059l.jpg" alt="Air" />
          <div class="status"><span class="bar"></span></div>
        </a>
        <a href="/play/air.XDBaJ" data-jtitle="Air" class="name">Air</a>
      </div>
    </div>
    <div class="item">
      <div class="inner">
        <a href="/play/one-piece-ita.XyZ12" class="poster">
          <img loading="lazy" src="https://img.animeworld.ac/locandine/onepiece.jpg" alt="One Piece (ITA)" />
        </a>
        <a href="/play/one-piece-ita.XyZ12" data-jtitle="One Piece" class="name">One Piece (ITA)</a>
      </div>
    </div>
  </div>
  <div id="sidebar">
    <a href="/play/top-anime.AAA" data-jtitle="Top Anime" class="name">Top Anime</a>
  </div>
</div>
</body></html>`

func TestParseSearchPageExtractsFullResultList(t *testing.T) {
	options := parseSearchPage([]byte(sampleSearchPage))

	if len(options) != 3 {
		t.Fatalf("expected 3 results, got %d: %+v", len(options), options)
	}
	if options[0].Key != "fairies-albums.71rpK" {
		t.Errorf("got first key %q", options[0].Key)
	}
	if options[1].Key != "air.XDBaJ" || options[1].Title != "Air" {
		t.Errorf("missing Air entry: %+v", options[1])
	}
	if options[1].Thumbnail != "https://img.animeworld.ac/locandine/22059l.jpg" {
		t.Errorf("got thumbnail %q", options[1].Thumbnail)
	}
	item, ok := options[1].ExtraData.(SearchItem)
	if !ok || item.JTitle != "Air" {
		t.Errorf("extra data not parsed: %+v", options[1].ExtraData)
	}
	for _, option := range options {
		if option.Key == "top-anime.AAA" {
			t.Error("sidebar entries must not be treated as search results")
		}
	}
}

func TestParseSearchPageDetectsItalianDubTitles(t *testing.T) {
	options := parseSearchPage([]byte(sampleSearchPage))

	item, ok := options[2].ExtraData.(SearchItem)
	if !ok {
		t.Fatalf("extra data not parsed: %+v", options[2].ExtraData)
	}
	if !item.Dub {
		t.Error(`"One Piece (ITA)" should be flagged as a dub`)
	}
}

func TestParseSearchPageReturnsEmptyWithoutResults(t *testing.T) {
	page := `<div class="film-list"><br><div class="alert alert-danger">Nessun risultato</div></div>`
	if options := parseSearchPage([]byte(page)); len(options) != 0 {
		t.Fatalf("expected no results, got %d", len(options))
	}
}

func TestMergeSearchResultsKeepsHtmlOrderAndUsesApiMetadata(t *testing.T) {
	htmlOptions := []providers.SelectionOption{
		{Key: "fairies-albums.71rpK", Title: "Fairies Albums", Label: "Fairies Albums"},
		{Key: "air-gear.RklhB", Title: "Air Gear", Label: "Air Gear"},
	}
	apiOptions := []providers.SelectionOption{
		{Key: "air-gear.RklhB", Title: "Air Gear", Label: "Air Gear (SUB ITA, 2006, 25 ep)", ExtraData: SearchItem{AnilistID: 857}},
	}

	merged := mergeSearchResults(htmlOptions, apiOptions)

	if len(merged) != 2 {
		t.Fatalf("expected 2 merged results, got %d", len(merged))
	}
	if merged[0].Key != "fairies-albums.71rpK" {
		t.Errorf("HTML order should be preserved, got %q first", merged[0].Key)
	}
	if merged[1].Label != "Air Gear (SUB ITA, 2006, 25 ep)" {
		t.Errorf("API metadata should win, got %q", merged[1].Label)
	}
	item, ok := merged[1].ExtraData.(SearchItem)
	if !ok || item.AnilistID != 857 {
		t.Errorf("API extra data should be kept: %+v", merged[1].ExtraData)
	}
}

func TestSortByExactTitleMovesExactMatchFirst(t *testing.T) {
	options := []providers.SelectionOption{
		{Title: "Fairies Albums"},
		{Title: "Air Gear"},
		{Title: "Air"},
	}

	sorted := sortByExactTitle(options, "Air")

	if sorted[0].Title != "Air" {
		t.Fatalf("expected exact title first, got %q", sorted[0].Title)
	}
	if sorted[1].Title != "Fairies Albums" || sorted[2].Title != "Air Gear" {
		t.Errorf("relative order of the other entries must be kept: %+v", sorted)
	}
}

func TestPlayKeyNormalisesWatchLinks(t *testing.T) {
	cases := map[string]string{
		"/play/air.XDBaJ":          "air.XDBaJ",
		"/play/air.XDBaJ/A7whj":    "air.XDBaJ",
		"https://x/play/air.XDBaJ": "air.XDBaJ",
		"/play/air.XDBaJ?x=1":      "air.XDBaJ",
		"/genre/action":            "",
		"":                         "",
	}
	for href, want := range cases {
		if got := playKey(href); got != want {
			t.Errorf("playKey(%q) = %q, want %q", href, got, want)
		}
	}
}

func TestSearchQueriesAddsSeasonNumberVariant(t *testing.T) {
	cases := map[string][]string{
		"Arifureta Shokugyou de Sekai Saikyou 3rd season": {"Arifureta Shokugyou de Sekai Saikyou 3rd season", "Arifureta Shokugyou de Sekai Saikyou 3"},
		"Kimetsu no Yaiba Season 2":                       {"Kimetsu no Yaiba Season 2", "Kimetsu no Yaiba 2"},
		"One Piece":                                       {"One Piece"},
	}
	for input, want := range cases {
		got := searchQueries(input)
		if len(got) != len(want) {
			t.Fatalf("searchQueries(%q) = %q, want %q", input, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("searchQueries(%q) = %q, want %q", input, got, want)
			}
		}
	}
}
