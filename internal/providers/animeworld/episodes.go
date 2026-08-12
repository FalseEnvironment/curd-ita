package animeworld

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// animeWorldServerID is the in-house server, the only one that exposes a direct
// media link rather than a third-party embed.
const animeWorldServerID = 9

const pageTTL = 5 * time.Minute

type episodeSource struct {
	dataID   string
	serverID int
	href     string
}

type episodeEntry struct {
	number  string
	numeric float64
	sources []episodeSource
}

type cachedPage struct {
	html      string
	fetchedAt time.Time
}

var pageCache sync.Map // showID -> cachedPage

var (
	divTagRE    = regexp.MustCompile(`(?is)<div[^>]*>`)
	anchorTagRE = regexp.MustCompile(`(?is)<a[^>]*>`)

	classAttrRE      = attrPattern("class")
	dataNameAttrRE   = attrPattern("data-name")
	episodeNumAttrRE = attrPattern("data-episode-num")
	dataIDAttrRE     = attrPattern("data-id")
	hrefAttrRE       = attrPattern("href")

	leadingNumberRE = regexp.MustCompile(`^(\d+(?:\.\d+)?)`)
)

func attrPattern(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?is)\b` + regexp.QuoteMeta(name) + `\s*=\s*["']([^"']*)["']`)
}

func attrValue(tag string, pattern *regexp.Regexp) string {
	match := pattern.FindStringSubmatch(tag)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func fetchPage(showID string, force bool) (string, error) {
	showID = strings.TrimSpace(showID)
	if showID == "" {
		return "", fmt.Errorf("empty show id")
	}

	if !force {
		if cached, ok := pageCache.Load(showID); ok {
			page := cached.(cachedPage)
			if time.Since(page.fetchedAt) < pageTTL {
				return page.html, nil
			}
		}
	}

	body, err := fetch(http.MethodGet, animeURL(showID))
	if err != nil {
		return "", err
	}
	html := string(body)
	if strings.Contains(html, "Errore 404") {
		return "", fmt.Errorf("animeworld page not found for %q", showID)
	}

	pageCache.Store(showID, cachedPage{html: html, fetchedAt: time.Now()})
	return html, nil
}

// parseEpisodes walks the watch page and groups every episode anchor by number,
// recording which server block it came from. Server blocks are plain divs
// carrying a data-name attribute, so anchors are attributed to the closest
// preceding block rather than by parsing nested markup.
func parseEpisodes(html string) []episodeEntry {
	type marker struct {
		offset   int
		serverID int
	}

	markers := make([]marker, 0, 8)
	for _, span := range divTagRE.FindAllStringIndex(html, -1) {
		tag := html[span[0]:span[1]]
		if !strings.Contains(strings.ToLower(attrValue(tag, classAttrRE)), "server") {
			continue
		}
		serverID, err := strconv.Atoi(attrValue(tag, dataNameAttrRE))
		if err != nil {
			continue
		}
		markers = append(markers, marker{offset: span[0], serverID: serverID})
	}

	serverAt := func(offset int) int {
		index := sort.Search(len(markers), func(i int) bool {
			return markers[i].offset > offset
		})
		if index == 0 {
			return 0
		}
		return markers[index-1].serverID
	}

	grouped := make(map[string]*episodeEntry)
	order := make([]string, 0, 32)

	for _, span := range anchorTagRE.FindAllStringIndex(html, -1) {
		tag := html[span[0]:span[1]]
		number := attrValue(tag, episodeNumAttrRE)
		if number == "" {
			continue
		}
		dataID := attrValue(tag, dataIDAttrRE)
		if dataID == "" {
			continue
		}

		entry, ok := grouped[number]
		if !ok {
			entry = &episodeEntry{number: number, numeric: numericValue(number)}
			grouped[number] = entry
			order = append(order, number)
		}
		entry.sources = append(entry.sources, episodeSource{
			dataID:   dataID,
			serverID: serverAt(span[0]),
			href:     attrValue(tag, hrefAttrRE),
		})
	}

	entries := make([]episodeEntry, 0, len(order))
	for _, number := range order {
		entries = append(entries, *grouped[number])
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].numeric < entries[j].numeric
	})
	return entries
}

// numericValue extracts a sortable number. AnimeWorld occasionally uses
// composite labels such as "5.5" or "268-269".
func numericValue(number string) float64 {
	match := leadingNumberRE.FindStringSubmatch(strings.TrimSpace(number))
	if len(match) < 2 {
		return 0
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}
	return value
}

func episodesList(showID, mode string) ([]string, error) {
	html, err := fetchPage(showID, false)
	if err != nil {
		return nil, err
	}

	entries := parseEpisodes(html)
	if len(entries) == 0 {
		// A stale cookie yields a stripped page; retry once with a fresh one.
		html, err = fetchPage(showID, true)
		if err != nil {
			return nil, err
		}
		entries = parseEpisodes(html)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("no episodes found for %q", showID)
	}

	numbers := make([]string, 0, len(entries))
	for _, entry := range entries {
		numbers = append(numbers, entry.number)
	}
	return numbers, nil
}

func findEpisode(showID string, epNo int) (episodeEntry, error) {
	entries, err := loadEpisodes(showID, false)
	if err != nil {
		return episodeEntry{}, err
	}

	entry, ok := matchEpisode(entries, epNo)
	if ok {
		return entry, nil
	}

	// The page may predate a newly released episode.
	entries, err = loadEpisodes(showID, true)
	if err != nil {
		return episodeEntry{}, err
	}
	if entry, ok = matchEpisode(entries, epNo); ok {
		return entry, nil
	}
	return episodeEntry{}, fmt.Errorf("episode %d not found for %q", epNo, showID)
}

func loadEpisodes(showID string, force bool) ([]episodeEntry, error) {
	html, err := fetchPage(showID, force)
	if err != nil {
		return nil, err
	}
	return parseEpisodes(html), nil
}

func matchEpisode(entries []episodeEntry, epNo int) (episodeEntry, bool) {
	target := strconv.Itoa(epNo)
	for _, entry := range entries {
		if entry.number == target {
			return entry, true
		}
	}
	for _, entry := range entries {
		if entry.numeric == float64(epNo) {
			return entry, true
		}
	}
	return episodeEntry{}, false
}
