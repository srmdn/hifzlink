// Package qfclient provides a lightweight client for the Quran Foundation API.
// It handles client_credentials token acquisition, caching, and verse fetching.
package qfclient

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client talks to the Quran Foundation Content API.
type Client struct {
	clientID     string
	clientSecret string
	authEndpoint string // e.g. https://prelive-oauth2.quran.foundation
	apiBase      string // e.g. https://apis-prelive.quran.foundation

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time

	cache   map[string]*VerseData
	cacheMu sync.RWMutex

	http *http.Client
}

// VerseData is the subset of QF verse data we use.
type VerseData struct {
	VerseKey    string
	TextUthmani string
	AudioURL    string // empty if no recitation available
}

// New creates a Client. Returns nil if any required field is empty.
func New(clientID, clientSecret, authEndpoint, apiBase string) *Client {
	if clientID == "" || clientSecret == "" || authEndpoint == "" || apiBase == "" {
		return nil
	}
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		authEndpoint: strings.TrimRight(authEndpoint, "/"),
		apiBase:      strings.TrimRight(apiBase, "/"),
		cache:        make(map[string]*VerseData),
		http:         &http.Client{Timeout: 8 * time.Second},
	}
}

// FetchVerse returns verse data for the given key (e.g. "1:1").
// Results are cached indefinitely for the lifetime of the process.
// recitationID 7 = Abdul Basit Murattal.
func (c *Client) FetchVerse(verseKey string, recitationID int) (*VerseData, error) {
	c.cacheMu.RLock()
	v, ok := c.cache[verseKey]
	c.cacheMu.RUnlock()
	if ok {
		return v, nil
	}

	token, err := c.token()
	if err != nil {
		return nil, fmt.Errorf("qfclient: get token: %w", err)
	}

	endpoint := fmt.Sprintf("%s/content/api/v4/verses/by_key/%s?fields=text_uthmani&audio=%d",
		c.apiBase, url.PathEscape(verseKey), recitationID)

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("qfclient: build request: %w", err)
	}
	req.Header.Set("x-auth-token", token)
	req.Header.Set("x-client-id", c.clientID)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qfclient: fetch verse: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qfclient: verse %s: HTTP %d", verseKey, resp.StatusCode)
	}

	var body struct {
		Verse struct {
			VerseKey    string `json:"verse_key"`
			TextUthmani string `json:"text_uthmani"`
			Audio       *struct {
				URL string `json:"url"`
			} `json:"audio"`
		} `json:"verse"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("qfclient: decode verse: %w", err)
	}

	const audioCDN = "https://verses.quran.com/"
	vd := &VerseData{
		VerseKey:    body.Verse.VerseKey,
		TextUthmani: body.Verse.TextUthmani,
	}
	if body.Verse.Audio != nil && body.Verse.Audio.URL != "" {
		audioURL := body.Verse.Audio.URL
		if !strings.HasPrefix(audioURL, "http") {
			audioURL = audioCDN + strings.TrimPrefix(audioURL, "/")
		}
		vd.AudioURL = audioURL
	}

	c.cacheMu.Lock()
	c.cache[verseKey] = vd
	c.cacheMu.Unlock()

	return vd, nil
}

// token returns a valid access token, refreshing if expired.
func (c *Client) token() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.accessToken, nil
	}

	tokenURL := c.authEndpoint + "/oauth2/token"
	vals := url.Values{
		"grant_type": {"client_credentials"},
		"scope":      {"content"},
	}

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(vals.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// QF requires client_secret_basic: credentials via HTTP Basic Auth.
	req.SetBasicAuth(c.clientID, c.clientSecret)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request: HTTP %d", resp.StatusCode)
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("decode token: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response")
	}

	c.accessToken = tok.AccessToken
	// Refresh 60s before actual expiry.
	expiry := tok.ExpiresIn
	if expiry <= 0 {
		expiry = 3600
	}
	c.tokenExpiry = time.Now().Add(time.Duration(expiry-60) * time.Second)
	log.Printf("qfclient: acquired new access token (expires in %ds)", expiry)
	return c.accessToken, nil
}
