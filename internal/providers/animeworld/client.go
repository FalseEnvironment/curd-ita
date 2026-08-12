package animeworld

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/wraient/curd/internal/curdhost"
)

const (
	userAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	defaultBaseURL = "https://www.animeworld.ac"
	// baseURLEnv overrides the site domain, which AnimeWorld rotates regularly.
	baseURLEnv = "CURD_ANIMEWORLD_BASE"
)

var baseURL = resolveBaseURL()

func resolveBaseURL() string {
	custom := strings.TrimSpace(os.Getenv(baseURLEnv))
	if custom == "" {
		return defaultBaseURL
	}
	if !strings.HasPrefix(custom, "http://") && !strings.HasPrefix(custom, "https://") {
		custom = "https://" + custom
	}
	return strings.TrimSuffix(custom, "/")
}

// AnimeWorld gates its API behind a cookie written by an inline script plus a
// csrf-token meta tag. Both are refreshed by loading the homepage.
var (
	sessionMu    sync.Mutex
	sessionReady bool

	csrfMu    sync.RWMutex
	csrfToken string
)

var (
	cookieScriptRE = regexp.MustCompile(`document\.cookie\s*=\s*["']([^"'=]+)=([^"';]+)`)
	metaTagRE      = regexp.MustCompile(`(?is)<meta[^>]*>`)
	contentAttrRE  = regexp.MustCompile(`(?is)\bcontent\s*=\s*["']([^"']*)["']`)
)

var fallbackClientOnce sync.Once
var fallbackClient *http.Client

func httpClient() *http.Client {
	if curdhost.HTTPClient != nil {
		if client := curdhost.HTTPClient(); client != nil {
			return client
		}
	}
	fallbackClientOnce.Do(func() {
		jar, _ := cookiejar.New(nil)
		fallbackClient = &http.Client{Timeout: 20 * time.Second, Jar: jar}
	})
	return fallbackClient
}

func currentCSRF() string {
	csrfMu.RLock()
	defer csrfMu.RUnlock()
	return csrfToken
}

func setCSRF(token string) {
	csrfMu.Lock()
	csrfToken = token
	csrfMu.Unlock()
}

func newRequest(method, rawURL string) (*http.Request, error) {
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", baseURL+"/")
	req.Header.Set("Accept-Language", "it-IT,it;q=0.9,en;q=0.8")
	if token := currentCSRF(); token != "" {
		req.Header.Set("csrf-token", token)
	}
	return req, nil
}

func do(method, rawURL string) ([]byte, error) {
	req, err := newRequest(method, rawURL)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if !curdhost.HTTPStatusOK(resp.StatusCode) {
		return nil, curdhost.HTTPStatusError("animeworld request", resp.StatusCode, body)
	}
	return body, nil
}

// ensureSession loads the homepage to pick up the anti-bot cookie and the
// csrf-token header. Pass force to refresh after a failed request.
func ensureSession(force bool) error {
	sessionMu.Lock()
	defer sessionMu.Unlock()

	if sessionReady && !force {
		return nil
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		body, err := do(http.MethodGet, baseURL+"/")
		if err != nil {
			lastErr = err
			continue
		}
		lastErr = nil

		if name, value, ok := findCookie(body); ok {
			storeCookie(name, value)
			// The cookie only takes effect on the next request, so reload.
			continue
		}
		if token, ok := findCSRFToken(body); ok {
			setCSRF(token)
			break
		}
		// Neither marker present: the site does not gate this request.
		break
	}
	if lastErr != nil {
		return lastErr
	}

	sessionReady = true
	return nil
}

func findCookie(body []byte) (name, value string, ok bool) {
	match := cookieScriptRE.FindSubmatch(body)
	if len(match) < 3 {
		return "", "", false
	}
	name = strings.TrimSpace(string(match[1]))
	value = strings.TrimSpace(string(match[2]))
	if name == "" || value == "" {
		return "", "", false
	}
	return name, value, true
}

func findCSRFToken(body []byte) (string, bool) {
	for _, tag := range metaTagRE.FindAll(body, -1) {
		if !strings.Contains(strings.ToLower(string(tag)), "csrf-token") {
			continue
		}
		match := contentAttrRE.FindSubmatch(tag)
		if len(match) < 2 {
			continue
		}
		if token := strings.TrimSpace(string(match[1])); token != "" {
			return token, true
		}
	}
	return "", false
}

func storeCookie(name, value string) {
	client := httpClient()
	if client.Jar == nil {
		return
	}
	parsed, err := url.Parse(baseURL + "/")
	if err != nil {
		return
	}
	client.Jar.SetCookies(parsed, []*http.Cookie{{Name: name, Value: value, Path: "/"}})
}

// fetch performs a request, refreshing the session once on failure. This
// mirrors the retry behaviour of the AnimeWorld-API reference implementation,
// where an expired cookie surfaces as a parse or status failure.
func fetch(method, rawURL string) ([]byte, error) {
	if err := ensureSession(false); err != nil {
		return nil, err
	}
	body, err := do(method, rawURL)
	if err == nil {
		return body, nil
	}
	if refreshErr := ensureSession(true); refreshErr != nil {
		return nil, err
	}
	return do(method, rawURL)
}

func animeURL(showID string) string {
	return baseURL + "/play/" + strings.TrimPrefix(strings.TrimSpace(showID), "/")
}
