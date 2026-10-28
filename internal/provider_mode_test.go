package internal

import "testing"

// modeAwareStub implements Provider plus ProviderModeMatcher so tests can force
// a stored-id/mode mismatch without touching the network.
type modeAwareStub struct {
	name        string
	matchResult bool
	matchErr    error
	searchFor   map[string][]SelectionOption
	searches    []string
}

func (s *modeAwareStub) Name() string { return s.name }

func (s *modeAwareStub) SearchAnime(query, mode string) ([]SelectionOption, error) {
	mode = normalizeTranslationType(mode)
	s.searches = append(s.searches, mode+":"+query)
	return s.searchFor[mode], nil
}

func (s *modeAwareStub) EpisodesList(showID, mode string) ([]string, error) { return nil, nil }

func (s *modeAwareStub) GetEpisodeURL(config CurdConfig, id string, epNo int) ([]string, error) {
	return nil, nil
}

func (s *modeAwareStub) ProviderIDMatchesMode(id, mode string) (bool, error) {
	return s.matchResult, s.matchErr
}

// modeAgnosticStub implements only the base Provider interface.
type modeAgnosticStub struct {
	name string
}

func (s *modeAgnosticStub) Name() string { return s.name }
func (s *modeAgnosticStub) SearchAnime(query, mode string) ([]SelectionOption, error) {
	return nil, nil
}
func (s *modeAgnosticStub) EpisodesList(showID, mode string) ([]string, error) { return nil, nil }
func (s *modeAgnosticStub) GetEpisodeURL(config CurdConfig, id string, epNo int) ([]string, error) {
	return nil, nil
}

func TestFindProviderIDForAnimeKeepsMatchingMode(t *testing.T) {
	stub := &modeAwareStub{name: "animeworld", matchResult: true}
	anime := &Anime{ProviderName: "animeworld", ProviderId: "ergo-proxy.bS-7W"}

	id, err := findProviderIDForAnime(stub, anime, "dub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "ergo-proxy.bS-7W" {
		t.Errorf("got %q, want stored id", id)
	}
	if len(stub.searches) != 0 {
		t.Errorf("expected no remap search, got %v", stub.searches)
	}
}

func TestFindProviderIDForAnimeRemapsOnModeMismatch(t *testing.T) {
	stub := &modeAwareStub{
		name:        "animeworld",
		matchResult: false,
		searchFor: map[string][]SelectionOption{
			"dub": {{Key: "ergo-proxy.bS-7W", Title: "Ergo Proxy (ITA)"}},
		},
	}
	anime := &Anime{
		Title:        AnimeTitle{English: "Ergo Proxy"},
		ProviderName: "animeworld",
		ProviderId:   "ergo-proxy-t.irMK5",
	}

	id, err := findProviderIDForAnime(stub, anime, "dub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "ergo-proxy.bS-7W" {
		t.Errorf("got %q, want remapped dub id", id)
	}
	if len(stub.searches) != 1 || stub.searches[0] != "dub:Ergo Proxy" {
		t.Errorf("expected one dub search, got %v", stub.searches)
	}
}

func TestFindProviderIDForAnimeKeepsIDWhenModeUnknown(t *testing.T) {
	// Providers without ModeMatcher are mode-agnostic: keep the stored id.
	stub := &modeAgnosticStub{name: "senshi"}
	anime := &Anime{ProviderName: "senshi", ProviderId: "62435"}

	id, err := findProviderIDForAnime(stub, anime, "dub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "62435" {
		t.Errorf("got %q, want stored id", id)
	}
}
