// Package qfclient provides a lightweight client for the Quran Foundation API.
// It handles client_credentials token acquisition, caching, and verse fetching.
package qfclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

// BookmarkData is a QF user bookmark (ayah-level).
type BookmarkData struct {
	ID        string
	SurahNum  int
	AyahNum   int
	CreatedAt string
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

// userAPIBase returns the base URL for user-scoped API endpoints.
func (c *Client) userAPIBase() string {
	return c.apiBase + "/auth/v1"
}

// AddBookmark saves an ayah as a QF bookmark for the given user.
// userToken is the user's OAuth2 access token from the session.
// mushaf 1 = Hafs An Asim (standard).
func (c *Client) AddBookmark(userToken string, surahNum, ayahNum int) (*BookmarkData, error) {
	body := map[string]any{
		"key":         surahNum,
		"type":        "ayah",
		"verseNumber": ayahNum,
		"mushaf":      1,
	}
	payload, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, c.userAPIBase()+"/bookmarks", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("qfclient: build bookmark request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-auth-token", userToken)
	req.Header.Set("x-client-id", c.clientID)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qfclient: add bookmark: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("qfclient: add bookmark: HTTP %d", resp.StatusCode)
	}

	var out struct {
		Success bool `json:"success"`
		Data    struct {
			ID          string `json:"id"`
			Key         int    `json:"key"`
			VerseNumber int    `json:"verseNumber"`
			CreatedAt   string `json:"createdAt"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("qfclient: decode bookmark: %w", err)
	}
	return &BookmarkData{
		ID:        out.Data.ID,
		SurahNum:  out.Data.Key,
		AyahNum:   out.Data.VerseNumber,
		CreatedAt: out.Data.CreatedAt,
	}, nil
}

// GetBookmarks returns all ayah bookmarks for the given user.
// userToken is the user's OAuth2 access token from the session.
func (c *Client) GetBookmarks(userToken string) ([]BookmarkData, error) {
	req, err := http.NewRequest(http.MethodGet, c.userAPIBase()+"/bookmarks?mushafId=1&first=20", nil)
	if err != nil {
		return nil, fmt.Errorf("qfclient: build get-bookmarks request: %w", err)
	}
	req.Header.Set("x-auth-token", userToken)
	req.Header.Set("x-client-id", c.clientID)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qfclient: get bookmarks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("qfclient: get bookmarks: HTTP %d: %s", resp.StatusCode, body)
	}

	var out struct {
		Data []struct {
			ID          string `json:"id"`
			Key         int    `json:"key"`
			VerseNumber int    `json:"verseNumber"`
			CreatedAt   string `json:"createdAt"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("qfclient: decode bookmarks: %w", err)
	}

	bookmarks := make([]BookmarkData, 0, len(out.Data))
	for _, d := range out.Data {
		bookmarks = append(bookmarks, BookmarkData{
			ID:        d.ID,
			SurahNum:  d.Key,
			AyahNum:   d.VerseNumber,
			CreatedAt: d.CreatedAt,
		})
	}
	return bookmarks, nil
}
