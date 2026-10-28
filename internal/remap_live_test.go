package internal

import (
	"os"
	"strings"
	"testing"
)

// TestLiveRemapStoredSubToDub reproduces the reported scenario: an anime was
// mapped to the AnimeWorld sub entry, the user later sets SubOrDub=dub, and
// playback must remap to the dub catalog entry instead of staying on sub.
func TestLiveRemapStoredSubToDub(t *testing.T) {
	if os.Getenv("CURD_LIVE") != "1" {
		t.Skip("set CURD_LIVE=1 to run live provider tests")
	}

	config := CurdConfig{
		Provider: `["animeworld"]`,
		SubOrDub: "dub",
	}
	SetGlobalConfig(&config)

	anime := &Anime{
		Title:         AnimeTitle{English: "Ergo Proxy", Romaji: "Ergo Proxy"},
		ProviderName:  "animeworld",
		ProviderId:    "ergo-proxy-t.irMK5", // stale SUB mapping
		TotalEpisodes: 23,
	}
	anime.Ep.Number = 1

	result, err := episodeModeResult(config, anime, 1, "dub")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	t.Logf("remapped provider id=%q mode=%q links=%v", result.ProviderID, result.Mode, result.Links)
	if result.ProviderID == "ergo-proxy-t.irMK5" {
		t.Fatal("provider id was not remapped away from the sub entry")
	}
	if len(result.Links) == 0 {
		t.Fatal("no links resolved")
	}
	for _, link := range result.Links {
		if strings.Contains(strings.ToUpper(link), "SUB") {
			t.Errorf("expected dub stream, got sub-looking link: %s", link)
		}
	}
}
