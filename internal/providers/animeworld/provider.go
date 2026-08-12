package animeworld

import "github.com/wraient/curd/internal/providers"

// Provider implements animeworld.ac catalog search and stream resolution.
//
// AnimeWorld publishes Italian subs and Italian dubs as separate catalog
// entries, so the audio mode is baked into the show ID at search time and the
// per-episode mode argument is only used as a hint.
type Provider struct{}

func (p *Provider) Name() string {
	return "animeworld"
}

func (p *Provider) SearchAnime(query, mode string) ([]providers.SelectionOption, error) {
	return searchAnime(query, mode)
}

func (p *Provider) EpisodesList(showID, mode string) ([]string, error) {
	return episodesList(showID, mode)
}

func (p *Provider) GetEpisodeURL(config providers.PlaybackConfig, id string, epNo int) ([]string, error) {
	links, _, err := p.GetEpisodeURLForModeWithHints(config, id, epNo, config.SubOrDub)
	return links, err
}

func (p *Provider) GetEpisodeURLForMode(config providers.PlaybackConfig, id string, epNo int, mode string) ([]string, error) {
	links, _, err := p.GetEpisodeURLForModeWithHints(config, id, epNo, mode)
	return links, err
}

func (p *Provider) GetEpisodeURLForModeWithHints(config providers.PlaybackConfig, id string, epNo int, mode string) ([]string, map[string]providers.StreamPlaybackHint, error) {
	return getEpisodeStreams(id, epNo)
}
