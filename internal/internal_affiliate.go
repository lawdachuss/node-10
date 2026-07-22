package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AffiliateModel represents a single model from the Chaturbate affiliate API.
// Endpoint: GET https://chaturbate.com/affiliates/api/onlinerooms/?format=json&wm={wm}
type AffiliateModel struct {
	Username     string `json:"username"`
	DisplayName  string `json:"display_name"`
	Gender       string `json:"gender"`
	Age          int    `json:"age"`
	NumUsers     int    `json:"num_users"`
	CurrentShow  string `json:"current_show"`
	ImageURL     string `json:"image_url"`
	ChatRoomURL  string `json:"chat_room_url"`
	RoomSubject  string `json:"room_subject"`
	IsHD         bool   `json:"is_hd"`
	IsNew        bool   `json:"is_new"`
	SecondsOnline int   `json:"seconds_online"`
	Tags         string `json:"tags"`
	Countries    string `json:"countries"`
}

// AffiliateAPIResult caches affiliate API results with a TTL.
type AffiliateAPIResult struct {
	mu       sync.RWMutex
	models   map[string]AffiliateModel
	fetchedAt time.Time
	ttl       time.Duration
}

var (
	affiliateCache = &AffiliateAPIResult{ttl: 30 * time.Second}
	affiliateClient = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:    2,
			IdleConnTimeout: 30 * time.Second,
		},
	}
)

// FetchAffiliateOnlineModels fetches all currently online models from the
// Chaturbate affiliate API. Results are cached for ttl duration.
// Returns the full API response as a map keyed by lowercased username.
// When the cache is fresh, returns cached data without a network call.
func FetchAffiliateOnlineModels(ctx context.Context, wmCode string) (map[string]AffiliateModel, error) {
	if wmCode == "" {
		return nil, fmt.Errorf("affiliate WM code is empty")
	}

	affiliateCache.mu.RLock()
	cached := affiliateCache.models
	cachedTime := affiliateCache.fetchedAt
	affiliateCache.mu.RUnlock()

	if cached != nil && time.Since(cachedTime) < affiliateCache.ttl {
		return cached, nil
	}

	affiliateCache.mu.Lock()
	defer affiliateCache.mu.Unlock()

	// Double-check after acquiring write lock
	if affiliateCache.models != nil && time.Since(affiliateCache.fetchedAt) < affiliateCache.ttl {
		return affiliateCache.models, nil
	}

	models, err := fetchAffiliateAPI(ctx, wmCode)
	if err != nil {
		// Return stale cache on error if we have it
		if affiliateCache.models != nil {
			return affiliateCache.models, nil
		}
		return nil, err
	}

	affiliateCache.models = models
	affiliateCache.fetchedAt = time.Now()
	return models, nil
}

func fetchAffiliateAPI(ctx context.Context, wmCode string) (map[string]AffiliateModel, error) {
	apiURL := fmt.Sprintf("https://chaturbate.com/affiliates/api/onlinerooms/?format=json&wm=%s", wmCode)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("affiliate: create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := affiliateClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("affiliate: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("affiliate: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("affiliate: read body: %w", err)
	}

	var models []AffiliateModel
	if err := json.Unmarshal(body, &models); err != nil {
		return nil, fmt.Errorf("affiliate: parse response: %w", err)
	}

	result := make(map[string]AffiliateModel, len(models))
	for _, m := range models {
		result[strings.ToLower(m.Username)] = m
	}

	return result, nil
}

// CheckAffiliateLive checks a single username against the affiliate API.
// Returns (isLive bool, currentShow string, error).
// This is the FASTEST liveness check - a single API call covers ALL channels.
// When the channel is NOT in the affiliate list, it is definitively offline.
// When it IS in the list, the currentShow tells us the type of broadcast.
func CheckAffiliateLive(ctx context.Context, wmCode, username string) (bool, string, error) {
	models, err := FetchAffiliateOnlineModels(ctx, wmCode)
	if err != nil {
		return false, "", err
	}

	model, found := models[strings.ToLower(username)]
	if !found {
		return false, "offline", nil
	}

	isLive := model.CurrentShow != "away" && model.CurrentShow != "offline"
	return isLive, model.CurrentShow, nil
}

// InvalidateAffiliateCache forces a re-fetch on the next call.
func InvalidateAffiliateCache() {
	affiliateCache.mu.Lock()
	defer affiliateCache.mu.Unlock()
	affiliateCache.models = nil
}
