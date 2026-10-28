package animeworld

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/wraient/curd/internal/providers"
)

type searchResponse struct {
	Animes []map[string]json.RawMessage `json:"animes"`
	Error  json.RawMessage              `json:"error"`
}

// AniList titles say "3rd season" or "Season 3" where AnimeWorld just appends
// the number, and the site search does not match the long form.
var (
	ordinalSeasonRE = regexp.MustCompile(`(?i)\b(\d+)(?:st|nd|rd|th)\s+season\b`)
	seasonNumberRE  = regexp.MustCompile(`(?i)\bseason\s+(\d+)\b`)
)

// The suggestion API (/api/search/v2) only returns the five most popular
// matches, so short titles such as "Air" are missing from it entirely. The HTML
// search page exposes the full result set, so it is the primary source; the API
// is merged in for the AniList/MAL ids, episode counts, and dub flag.
const (
	filmListMarker = `<div class="film-list">`
	sidebarMarker  = `id="sidebar"`
	itemMarker     = `<div class="item">`
)

var (
	posterAnchorRE = regexp.MustCompile(`(?is)<a[^>]*\bclass\s*=\s*["']poster["'][^>]*>`)
	nameAnchorRE   = regexp.MustCompile(`(?is)<a[^>]*\bclass\s*=\s*["']name["'][^>]*>(.*?)</a>`)
	imgTagRE       = regexp.MustCompile(`(?is)<img[^>]*>`)
	htmlTagRE      = regexp.MustCompile(`(?s)<[^>]*>`)

	jtitleAttrRE = attrPattern("data-jtitle")
	srcAttrRE    = attrPattern("src")
)

// searchQueries returns the query followed by a season-number variant, if any.
func searchQueries(query string) []string {
	variant := ordinalSeasonRE.ReplaceAllString(query, "$1")
	variant = seasonNumberRE.ReplaceAllString(variant, "$1")
	variant = strings.Join(strings.Fields(variant), " ")
	if variant == "" || variant == query {
		return []string{query}
	}
	return []string{query, variant}
}

func searchAnime(query, mode string) ([]providers.SelectionOption, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty search query")
	}

	var options []providers.SelectionOption
	usedQuery := query
	for _, candidate := range searchQueries(query) {
		found, err := searchOnce(candidate)
		if err != nil {
			return nil, err
		}
		if len(found) > 0 {
			options = found
			usedQuery = candidate
			break
		}
	}
	if len(options) == 0 {
		return nil, fmt.Errorf("no results for %q", query)
	}

	options = sortByExactTitle(options, usedQuery)
	return filterByMode(options, mode), nil
}

func searchOnce(query string) ([]providers.SelectionOption, error) {
	pageOptions, pageErr := searchPageResults(query)
	apiOptions, apiErr := searchAPIResults(query)

	if pageErr == nil && len(pageOptions) > 0 {
		return mergeSearchResults(pageOptions, apiOptions), nil
	}
	if len(apiOptions) > 0 {
		return apiOptions, nil
	}
	if pageErr != nil {
		return nil, pageErr
	}
	if apiErr != nil {
		return nil, apiErr
	}
	return nil, nil
}

func searchPageResults(query string) ([]providers.SelectionOption, error) {
	rawURL := fmt.Sprintf("%s/search?keyword=%s", baseURL, url.QueryEscape(query))
	body, err := fetch(http.MethodGet, rawURL)
	if err != nil {
		return nil, err
	}
	return parseSearchPage(body), nil
}

func searchAPIResults(query string) ([]providers.SelectionOption, error) {
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
	return options, nil
}

// parseSearchPage walks the full search page and extracts the entries inside
// the film list. Sidebar widgets ("Top Anime", "Nuove aggiunte") also link to
// /play/ pages, so parsing is confined to the results section.
func parseSearchPage(body []byte) []providers.SelectionOption {
	document := string(body)
	start := strings.Index(document, filmListMarker)
	if start < 0 {
		return nil
	}
	section := document[start:]
	if end := strings.Index(section, sidebarMarker); end >= 0 {
		section = section[:end]
	}

	chunks := strings.Split(section, itemMarker)
	options := make([]providers.SelectionOption, 0, len(chunks))
	for _, chunk := range chunks[1:] {
		option, ok := parseSearchItem(chunk)
		if !ok {
			continue
		}
		options = append(options, option)
	}
	return options
}

func parseSearchItem(chunk string) (providers.SelectionOption, bool) {
	nameAnchor := nameAnchorRE.FindStringSubmatch(chunk)
	if nameAnchor == nil {
		return providers.SelectionOption{}, false
	}

	key := playKey(attrValue(posterAnchorRE.FindString(chunk), hrefAttrRE))
	if key == "" {
		key = playKey(attrValue(nameAnchor[0], hrefAttrRE))
	}
	if key == "" {
		return providers.SelectionOption{}, false
	}

	jtitle := strings.TrimSpace(html.UnescapeString(attrValue(nameAnchor[0], jtitleAttrRE)))
	name := strings.TrimSpace(html.UnescapeString(htmlTagRE.ReplaceAllString(nameAnchor[1], "")))
	if name == "" {
		name = jtitle
	}
	if name == "" {
		return providers.SelectionOption{}, false
	}

	thumbnail := ""
	if img := imgTagRE.FindString(chunk); img != "" {
		thumbnail = strings.TrimSpace(html.UnescapeString(attrValue(img, srcAttrRE)))
	}

	return providers.SelectionOption{
		Key:       key,
		Label:     name,
		Title:     name,
		Thumbnail: thumbnail,
		// AnimeWorld suffixes dubbed entries with "(ITA)", which is the only
		// dub marker available when the API does not return this entry.
		ExtraData: SearchItem{Name: name, JTitle: jtitle, Dub: isDubTitle(name)},
	}, true
}

func isDubTitle(name string) bool {
	return strings.Contains(strings.ToUpper(name), "(ITA)")
}

// playKey normalises a watch link to the "slug.identifier" form used as the
// provider id, dropping any episode segment, query string, or host.
func playKey(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if idx := strings.IndexAny(href, "?#"); idx >= 0 {
		href = href[:idx]
	}
	if idx := strings.Index(href, "/play/"); idx >= 0 {
		href = href[idx+len("/play/"):]
	} else {
		href = strings.TrimPrefix(href, "play/")
	}
	href = strings.Trim(href, "/")
	if idx := strings.Index(href, "/"); idx >= 0 {
		href = href[:idx]
	}
	if !strings.Contains(href, ".") {
		return ""
	}
	return href
}

// mergeSearchResults keeps the order of the full HTML result list while taking
// richer metadata (ids, episode count, dub flag, label) from the API for the
// entries it happens to return.
func mergeSearchResults(ordered, extra []providers.SelectionOption) []providers.SelectionOption {
	if len(extra) == 0 {
		return ordered
	}

	rich := make(map[string]providers.SelectionOption, len(extra))
	for _, option := range extra {
		rich[option.Key] = option
	}

	merged := make([]providers.SelectionOption, 0, len(ordered)+len(extra))
	seen := make(map[string]bool, len(ordered)+len(extra))
	for _, option := range ordered {
		seen[option.Key] = true
		if metadata, ok := rich[option.Key]; ok {
			merged = append(merged, metadata)
			continue
		}
		merged = append(merged, option)
	}
	for _, option := range extra {
		if seen[option.Key] {
			continue
		}
		seen[option.Key] = true
		merged = append(merged, option)
	}
	return merged
}

// sortByExactTitle moves entries whose title is exactly the query to the front.
// AnimeWorld orders the search page by popularity, so a short title like "Air"
// can sit dozens of rows down even though it is the obvious match.
func sortByExactTitle(options []providers.SelectionOption, query string) []providers.SelectionOption {
	want := normalizeTitle(query)
	if want == "" {
		return options
	}
	sort.SliceStable(options, func(i, j int) bool {
		return isExactTitle(options[i], want) && !isExactTitle(options[j], want)
	})
	return options
}

func isExactTitle(option providers.SelectionOption, want string) bool {
	if normalizeTitle(option.Title) == want {
		return true
	}
	item, ok := option.ExtraData.(SearchItem)
	return ok && normalizeTitle(item.JTitle) == want
}

func normalizeTitle(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	replacer := strings.NewReplacer(":", "", "-", " ", "_", " ", ".", " ", "'", "", "\"", "", "!", "", "?", "", "(", " ", ")", " ")
	title = replacer.Replace(title)
	return strings.Join(strings.Fields(title), " ")
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

// filterByMode returns only entries matching the requested audio mode.
// AnimeWorld lists dubs as separate catalog entries, so we filter to the
// requested mode. If no entries match, we return an empty list so the caller
// can decide whether to fall back to the other mode.
func filterByMode(options []providers.SelectionOption, mode string) []providers.SelectionOption {
	wantDub := providers.NormalizeTranslationType(mode) == "dub"
	filtered := make([]providers.SelectionOption, 0, len(options))
	for _, option := range options {
		if matchesMode(option, wantDub) {
			filtered = append(filtered, option)
		}
	}
	return filtered
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
