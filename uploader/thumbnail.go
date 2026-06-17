package uploader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ThumbnailUploader handles uploading thumbnail images to Pixhost.to
type ThumbnailUploader struct {
	apiKey string // Not used for Pixhost, kept for compatibility
	client *http.Client
}

// pixhostResponse is the JSON response from the Pixhost.to API
type pixhostResponse struct {
	Name    string `json:"name"`
	ShowURL string `json:"show_url"`
	ThURL   string `json:"th_url"`
	ImgURL  string `json:"img_url"` // Direct image URL (may be empty for NSFW)
}

// NewThumbnailUploader creates a new Pixhost.to thumbnail uploader.
// apiKey parameter is ignored (Pixhost doesn't require API keys)
func NewThumbnailUploader(apiKey string) *ThumbnailUploader {
	return &ThumbnailUploader{
		apiKey: apiKey,
		client: newNoProxyClient(2 * time.Minute),
	}
}

// Upload uploads a thumbnail image to Pixhost.to and returns the direct image URL.
// Pixhost only supports JPEG, PNG, and GIF — WebP files are rejected with a fast
// error so the caller can fall through to hosts that support WebP (e.g. Catbox.moe).
func (t *ThumbnailUploader) Upload(thumbnailPath string) (string, error) {
	log.Printf("Uploading thumbnail to Pixhost.to: %s", thumbnailPath)

	// ── Fast-fail for WebP ─────────────────────────────────────────────────
	// Pixhost returns 414 "Unexpected File Format" for WebP. Skip immediately
	// so the caller's fallback chain (→ Catbox.moe) handles it without delay.
	if strings.EqualFold(filepath.Ext(thumbnailPath), ".webp") {
		return "", fmt.Errorf("Pixhost does not support WebP format — use a host that supports it (e.g. Catbox.moe)")
	}

	fileData, err := os.ReadFile(thumbnailPath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	// Try each upload strategy in order until one succeeds.
	// Strategy 1: multipart/form-data with fields before file (Pixhost prefers this order)
	// Strategy 2: raw binary body with proper Content-Type (fallback for CDN issues)
	var lastErr error
	strategies := []struct {
		name string
		fn   func() (*http.Response, error)
	}{
		{"multipart", func() (*http.Response, error) {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			// Write form fields FIRST (some APIs require fields before file part)
			if err := writer.WriteField("content_type", "1"); err != nil {
				return nil, fmt.Errorf("write content_type: %w", err)
			}
			if err := writer.WriteField("max_th_size", "420"); err != nil {
				return nil, fmt.Errorf("write max_th_size: %w", err)
			}

			part, err := writer.CreateFormFile("img", filepath.Base(thumbnailPath))
			if err != nil {
				return nil, fmt.Errorf("create form file: %w", err)
			}
			if _, err := io.Copy(part, bytes.NewReader(fileData)); err != nil {
				return nil, fmt.Errorf("copy file: %w", err)
			}
			if err := writer.Close(); err != nil {
				return nil, fmt.Errorf("close writer: %w", err)
			}

			req, err := http.NewRequest("POST", "https://api.pixhost.to/images", &buf)
			if err != nil {
				return nil, fmt.Errorf("create request: %w", err)
			}
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("Accept", "application/json")
			req.Header.Set("User-Agent", defaultUserAgent)
			return t.client.Do(req)
		}},
		{"raw body", func() (*http.Response, error) {
			// Fallback: send raw file bytes with Content-Type set to the image type.
			// Some CDNs handle raw body uploads better than multipart.
			ext := strings.ToLower(filepath.Ext(thumbnailPath))
			mimeType := "application/octet-stream"
			switch ext {
			case ".webp":
				mimeType = "image/webp"
			case ".jpg", ".jpeg":
				mimeType = "image/jpeg"
			case ".png":
				mimeType = "image/png"
			}

			req, err := http.NewRequest("POST", "https://api.pixhost.to/images", bytes.NewReader(fileData))
			if err != nil {
				return nil, fmt.Errorf("create raw request: %w", err)
			}
			req.Header.Set("Content-Type", mimeType)
			req.Header.Set("Accept", "application/json")
			req.Header.Set("User-Agent", defaultUserAgent)
			return t.client.Do(req)
		}},
	}

	var resp *http.Response
	for _, strat := range strategies {
		resp, err = strat.fn()
		if err != nil {
			lastErr = fmt.Errorf("%s: send request: %w", strat.name, err)
			log.Printf("Pixhost: %s failed (request error) — trying next strategy: %v", strat.name, err)
			continue
		}
		// Only fall through to next strategy on 414 — other status codes are handled downstream
		if resp.StatusCode == 414 {
			bodyDump, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("%s: Pixhost returned status 414: %s", strat.name, strings.TrimSpace(string(bodyDump)))
			log.Printf("Pixhost: %s got 414 — trying next strategy", strat.name)
			continue
		}
		break
	}

	if resp == nil {
		return "", fmt.Errorf("all upload strategies failed, last: %w", lastErr)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Pixhost returned status %d: %s", resp.StatusCode, string(body))
	}

	var result pixhostResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	// For NSFW uploads img_url is always empty. Derive the full-size CDN URL
	// from th_url by replacing the thumbnail path with the images path:
	//   https://t2.pixhost.to/thumbs/ID/file.jpg
	//   → https://img2.pixhost.to/images/ID/file.jpg
	// This gives the original full-resolution image without any age-gate.
	// th_url is only used as a last resort because it is capped at max_th_size.
	imageURL := strings.TrimSpace(result.ImgURL)
	if imageURL == "" {
		if th := strings.TrimSpace(result.ThURL); th != "" {
			imageURL = pixhostThumbToFull(th)
		}
	}
	if imageURL == "" && strings.Contains(result.ShowURL, "/show/") {
		imageURL = strings.Replace(result.ShowURL, "/show/", "/images/", 1)
	}
	if imageURL == "" {
		return "", fmt.Errorf("Pixhost returned no image URL (response: %s)", string(body))
	}
	log.Printf("Pixhost response: img_url=%q show_url=%q th_url=%q → using %q",
		result.ImgURL, result.ShowURL, result.ThURL, imageURL)

	log.Printf("Thumbnail uploaded to Pixhost: %s", imageURL)
	return imageURL, nil
}

// pixhostThToFull re-derives the full-resolution CDN URL from a Pixhost
// thumbnail URL.
//
//	https://t2.pixhost.to/thumbs/8020/file.jpg
//	→ https://img2.pixhost.to/images/8020/file.jpg
//
// If the URL doesn't match the expected pattern, it is returned unchanged so
// we always have something to store rather than an empty string.
var pixhostThRe = regexp.MustCompile(`^https?://t(\d+)\.pixhost\.to/thumbs/`)

func pixhostThumbToFull(thURL string) string {
	loc := pixhostThRe.FindStringIndex(thURL)
	if loc == nil {
		return thURL
	}
	// Extract the server number from the match (group 1)
	sub := pixhostThRe.FindStringSubmatch(thURL)
	if len(sub) < 2 {
		return thURL
	}
	n := sub[1]
	full := "https://img" + n + ".pixhost.to/images/" + thURL[loc[1]:]
	return full
}
