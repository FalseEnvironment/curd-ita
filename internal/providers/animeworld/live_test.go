package animeworld

import (
	"os"
	"strconv"
	"testing"

	"github.com/wraient/curd/internal/providers"
)

// TestLiveEndToEnd hits animeworld.ac. Enable with CURD_LIVE=1 and optionally
// override the query with CURD_LIVE_QUERY.
func TestLiveEndToEnd(t *testing.T) {
	if os.Getenv("CURD_LIVE") != "1" {
		t.Skip("set CURD_LIVE=1 to run live provider tests")
	}

	query := os.Getenv("CURD_LIVE_QUERY")
	if query == "" {
		query = "bocchi the rock"
	}

	provider := &Provider{}

	options, err := provider.SearchAnime(query, "sub")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	t.Logf("%d results, first: key=%q label=%q extra=%+v", len(options), options[0].Key, options[0].Label, options[0].ExtraData)

	episodes, err := provider.EpisodesList(options[0].Key, "sub")
	if err != nil {
		t.Fatalf("episodes: %v", err)
	}
	t.Logf("%d episodes: %v", len(episodes), episodes)

	epNo, err := strconv.Atoi(episodes[0])
	if err != nil {
		t.Skipf("first episode %q is not a plain number", episodes[0])
	}

	links, hints, err := provider.GetEpisodeURLForModeWithHints(providers.PlaybackConfig{SubOrDub: "sub"}, options[0].Key, epNo, "sub")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	t.Logf("stream: %v (hints: %+v)", links, hints)
}
