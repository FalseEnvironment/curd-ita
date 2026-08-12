package animeworld

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/wraient/curd/internal/providers"
)

type episodeInfoResponse struct {
	Grabber string `json:"grabber"`
	Name    string `json:"name"`
	Target  string `json:"target"`
}

var directMediaExtensions = []string{".mp4", ".m3u8", ".mkv", ".webm"}

func getEpisodeStreams(showID string, epNo int) ([]string, map[string]providers.StreamPlaybackHint, error) {
	showID = strings.TrimSpace(showID)
	if showID == "" {
		return nil, nil, fmt.Errorf("empty show id")
	}
	if epNo <= 0 {
		return nil, nil, fmt.Errorf("invalid episode number %d", epNo)
	}

	entry, err := findEpisode(showID, epNo)
	if err != nil {
		return nil, nil, err
	}

	var lastErr error
	for _, source := range prioritizeSources(entry.sources) {
		link, err := resolveGrabber(source.dataID)
		if err != nil {
			lastErr = err
			continue
		}
		// Third-party servers hand back an embed page, which mpv cannot play.
		if source.serverID != animeWorldServerID && !isDirectMedia(link) {
			continue
		}
		hints := map[string]providers.StreamPlaybackHint{
			link: {Referrer: baseURL + "/"},
		}
		return []string{link}, hints, nil
	}

	if lastErr != nil {
		return nil, nil, fmt.Errorf("no playable stream for episode %d: %w", epNo, lastErr)
	}
	return nil, nil, fmt.Errorf("no playable stream for episode %d", epNo)
}

// prioritizeSources puts the in-house server first, since it is the only one
// that returns a direct media link.
func prioritizeSources(sources []episodeSource) []episodeSource {
	ordered := make([]episodeSource, len(sources))
	copy(ordered, sources)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].serverID == animeWorldServerID && ordered[j].serverID != animeWorldServerID
	})
	return ordered
}

func resolveGrabber(dataID string) (string, error) {
	rawURL := fmt.Sprintf("%s/api/episode/info?id=%s&alt=0", baseURL, url.QueryEscape(dataID))
	body, err := fetch(http.MethodGet, rawURL)
	if err != nil {
		return "", err
	}

	var payload episodeInfoResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("parse animeworld episode info: %w", err)
	}
	link := strings.TrimSpace(payload.Grabber)
	if link == "" {
		return "", fmt.Errorf("no grabber link for episode source %s", dataID)
	}
	return link, nil
}

func isDirectMedia(link string) bool {
	path := link
	if parsed, err := url.Parse(link); err == nil {
		path = parsed.Path
	}
	path = strings.ToLower(path)
	for _, extension := range directMediaExtensions {
		if strings.HasSuffix(path, extension) {
			return true
		}
	}
	return false
}
