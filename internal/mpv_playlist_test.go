package internal

import (
	"strings"
	"testing"
)

func TestEpisodeLabelMarksCurrentAndFiller(t *testing.T) {
	c := &MPVPlaylistController{
		anime: &Anime{
			Title:          AnimeTitle{English: "Demo"},
			FillerEpisodes: []int{3, 5},
			Ep:             Episode{Number: 2},
		},
		preferredMode:  "sub",
		currentPlaying: 2,
		currentMode:    "sub",
	}
	label := c.episodeLabel(2, "sub")
	if !strings.Contains(label, "Ep 02") || !strings.Contains(label, "◀") {
		t.Fatalf("current ep label: %q", label)
	}
	filler := c.episodeLabel(3, "sub")
	if !strings.Contains(filler, "Filler") {
		t.Fatalf("expected filler mark, got %q", filler)
	}
	dub := c.episodeLabel(2, "dub")
	if !strings.Contains(strings.ToUpper(dub), "DUB") {
		t.Fatalf("expected dub mark, got %q", dub)
	}
}

func TestEpisodeIndexFindsAlternateModeSlot(t *testing.T) {
	c := &MPVPlaylistController{
		slots: []playlistSlot{
			{Episode: 1, Mode: "sub"},
			{Episode: 2, Mode: "sub"},
			{Episode: 2, Mode: "dub"},
			{Episode: 3, Mode: "sub"},
		},
	}
	if got := c.episodeIndex(2, "dub"); got != 2 {
		t.Fatalf("dub slot index=%d want 2", got)
	}
	if got := c.episodeIndex(2, "sub"); got != 1 {
		t.Fatalf("sub slot index=%d want 1", got)
	}
	if got := c.episodeIndex(9, "sub"); got != 8 {
		t.Fatalf("fallback index for ep9=%d want 8", got)
	}
}

func TestIsEpisodeFillerHelper(t *testing.T) {
	if !IsEpisodeFiller([]int{1, 4, 7}, 4) {
		t.Fatal("expected filler")
	}
	if IsEpisodeFiller([]int{1, 4, 7}, 3) {
		t.Fatal("expected non-filler")
	}
}
