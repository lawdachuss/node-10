package uploader

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const imgbbAPIURL = "https://api.imgbb.com/1/upload"

type imgbbResponse struct {
	Data struct {
		URL string `json:"url"`
	} `json:"data"`
	Status int             `json:"status"`
	Error  json.RawMessage `json:"error,omitempty"`
}

type ImgBBUploader struct {
	apiKey string
	client *http.Client
}

func NewImgBBUploader() *ImgBBUploader {
	key := os.Getenv("IMGBB_API_KEY")
	return &ImgBBUploader{
		apiKey: key,
		client: newNoProxyClient(60 * time.Second),
	}
}

func (u *ImgBBUploader) Upload(filePath string) (string, error) {
	if u.apiKey == "" {
		return "", fmt.Errorf("imgbb: IMGBB_API_KEY not set")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("imgbb: read file: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	form := url.Values{
		"key":   {u.apiKey},
		"image": {encoded},
	}

	// Retry on rate-limit errors with exponential backoff
	maxAttempts := 5
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second // 2s, 4s, 8s, 16s
			time.Sleep(backoff)
		}

		resp, err := u.client.PostForm(imgbbAPIURL, form)
		if err != nil {
			lastErr = fmt.Errorf("imgbb: post: %w", err)
			if attempt < maxAttempts {
				continue
			}
			return "", lastErr
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		resp.Body.Close() // close immediately — no defer inside loop
		if err != nil {
			lastErr = fmt.Errorf("imgbb: read response: %w", err)
			if attempt < maxAttempts {
				continue
			}
			return "", lastErr
		}

		// Check for HTTP-level rate limiting (429)
		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("imgbb: rate limited (HTTP 429)")
			if attempt < maxAttempts {
				continue
			}
			return "", lastErr
		}

		var result imgbbResponse
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = fmt.Errorf("imgbb: parse response: %w", err)
			if attempt < maxAttempts {
				continue
			}
			return "", lastErr
		}

		if result.Status != 200 {
			msg := string(result.Error)
			// ImgBB error is an object like {"message":"...","code":...}; extract message if possible.
			var errObj struct {
				Message string `json:"message"`
			}
			if json.Unmarshal(result.Error, &errObj) == nil && errObj.Message != "" {
				msg = errObj.Message
			}
			if msg == "" || msg == "null" {
				msg = string(body)
			}
			err = fmt.Errorf("imgbb: error: %s", msg)
			lastErr = err
			// Retry on rate-limit error messages
			if strings.Contains(strings.ToLower(msg), "rate limit") {
				if attempt < maxAttempts {
					continue
				}
				return "", lastErr
			}
			// Non-rate-limit errors are fatal — don't retry
			return "", err
		}

		if result.Data.URL == "" {
			return "", fmt.Errorf("imgbb: empty image URL in response")
		}

		return result.Data.URL, nil
	}

	return "", lastErr
}
