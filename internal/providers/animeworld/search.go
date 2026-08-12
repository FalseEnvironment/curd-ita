package animeworld

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/wraient/curd/internal/providers"
)

type searchResponse struct {
	Animes []map[string]json.RawMessage `json:"animes"`
	Error  json.RawMessage              `json:"error"`
}

func searchAnime(query, mode string) ([]providers.SelectionOption, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty search query")
	}

	rawURL := fmt.Sprintf("%s/api/search/v2?keyword=%s", baseURL, url.QueryEscape(query))
	body, err := fetch(http.MethodPost, rawURL)
	if err != nil {
		return nil, err
	}

	var payload searchResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse animeworld search response: %w", err)
	}
	if len(payload.Error) > 0 && string(payload.Error) != "null" {
		return nil, fmt.Errorf("animeworld search failed: %s", strings.Trim(string(payload.Error), `"`))
	}

	options := make([]providers.SelectionOption, 0, len(payload.Animes))
	for _, entry := range payload.Animes {
		option, ok := toSelectionOption(entry)
		if !ok {
			continue
		}
		options = append(options, option)
	}
	if len(options) == 0 {
		return nil, fmt.Errorf("no results for %q", query)
	}

	return sortByMode(options, mode), nil
}

func toSelectionOption(entry map[string]json.RawMessage) (providers.SelectionOption, bool) {
	// AnimeWorld builds the watch path from two separate fields.
	slug := jsonString(entry, "link")
	identifier := jsonString(entry, "identifier")
	if slug == "" || identifier == "" {
		return providers.SelectionOption{}, false
	}

	item := SearchItem{
		ID:        jsonInt(entry, "id"),
		Name:      jsonString(entry, "name"),
		JTitle:    jsonString(entry, "jtitle"),
		Episodes:  jsonInt(entry, "episodes"),
		Dub:       jsonBool(entry, "dub"),
		Year:      jsonString(entry, "year"),
		Studio:    jsonString(entry, "studio"),
		MalID:     jsonInt(entry, "malId"),
		AnilistID: jsonInt(entry, "anilistId"),
	}
	if item.Name == "" {
		item.Name = slug
	}

	return providers.SelectionOption{
		Key:       slug + "." + identifier,
		Label:     buildLabel(item),
		Title:     item.Name,
		Thumbnail: jsonString(entry, "image"),
		ExtraData: item,
	}, true
}

func buildLabel(item SearchItem) string {
	label := item.Name

	details := make([]string, 0, 3)
	if item.Dub {
		details = append(details, "ITA dub")
	} else {
		details = append(details, "SUB ITA")
	}
	if item.Year != "" {
		details = append(details, item.Year)
	}
	if item.Episodes > 0 {
		details = append(details, fmt.Sprintf("%d ep", item.Episodes))
	}
	return label + " (" + strings.Join(details, ", ") + ")"
}

// sortByMode moves entries matching the requested audio to the front. It never
// drops the others: AnimeWorld lists dubs as separate entries, so a missing dub
// should fall back to the sub rather than produce an empty result.
func sortByMode(options []providers.SelectionOption, mode string) []providers.SelectionOption {
	wantDub := providers.NormalizeTranslationType(mode) == "dub"
	sort.SliceStable(options, func(i, j int) bool {
		return matchesMode(options[i], wantDub) && !matchesMode(options[j], wantDub)
	})
	return options
}

func matchesMode(option providers.SelectionOption, wantDub bool) bool {
	item, ok := option.ExtraData.(SearchItem)
	if !ok {
		return false
	}
	return item.Dub == wantDub
}

func jsonString(entry map[string]json.RawMessage, key string) string {
	raw, ok := entry[key]
	if !ok {
		return ""
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		// The API uses "??" as a placeholder for unknown values.
		if typed == "??" {
			return ""
		}
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		return ""
	}
}

func jsonInt(entry map[string]json.RawMessage, key string) int {
	value := jsonString(entry, key)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return int(parsed)
}

func jsonBool(entry map[string]json.RawMessage, key string) bool {
	raw, ok := entry[key]
	if !ok {
		return false
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case string:
		return typed != "" && typed != "0" && typed != "false"
	default:
		return false
	}
}
