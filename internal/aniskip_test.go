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

func TestParseAniSkipResponseUsesSkipTypeForEndingOnlyEpisodes(t *testing.T) {
	anime := &Anime{}
	response := `{"found":true,"results":[{"interval":{"start_time":1334.15,"end_time":1419.59},"skip_type":"ed","skip_id":"x","episode_length":1420.18}]}`

	if err := ParseAniSkipResponse(response, anime, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if anime.Ep.SkipTimes.Op != (Skip{}) {
		t.Fatalf("ending stored as opening: %+v", anime.Ep.SkipTimes.Op)
	}
	if anime.Ep.SkipTimes.Ed != (Skip{Start: 1334, End: 1420}) {
		t.Fatalf("ending = %+v, want {1334 1420}", anime.Ep.SkipTimes.Ed)
	}
}

func TestParseAniSkipResponseReadsOpeningAndEndingInAnyOrder(t *testing.T) {
	anime := &Anime{}
	response := `{"found":true,"results":[{"interval":{"start_time":1370,"end_time":1420},"skip_type":"ed"},{"interval":{"start_time":109,"end_time":199},"skip_type":"op"}]}`

	if err := ParseAniSkipResponse(response, anime, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := SkipTimes{Op: Skip{Start: 109, End: 199}, Ed: Skip{Start: 1370, End: 1420}}
	if anime.Ep.SkipTimes != want {
		t.Fatalf("skip times = %+v, want %+v", anime.Ep.SkipTimes, want)
	}
}
