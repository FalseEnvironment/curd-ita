package internal

import "testing"

func TestParseAniSkipResponseClearsPreviousEpisodeTimes(t *testing.T) {
	anime := &Anime{}
	anime.Ep.SkipTimes = SkipTimes{
		Op: Skip{Start: 93, End: 183},
		Ed: Skip{Start: 1373, End: 1419},
	}

	err := ParseAniSkipResponse(`{"found":false,"results":[]}`, anime, 0)

	if err == nil {
		t.Fatal("expected an error for a response without skip times")
	}
	if anime.Ep.SkipTimes != (SkipTimes{}) {
		t.Fatalf("stale skip times kept: %+v", anime.Ep.SkipTimes)
	}
}
