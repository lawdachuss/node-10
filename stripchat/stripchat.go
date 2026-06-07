package stripchat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/teacat/chaturbate-dvr/internal"
	"github.com/teacat/chaturbate-dvr/site"
)

type StripchatSite struct{}

func NewStripchatSite() *StripchatSite {
	return &StripchatSite{}
}

type camResponse struct {
	Cam struct {
		StreamName        string            `json:"streamName"`
		IsCamActive       bool              `json:"isCamActive"`
		ViewServers       map[string]string `json:"viewServers"`
		BroadcastSettings struct {
			BroadcastType string `json:"broadcastType"`
		} `json:"broadcastSettings"`
		Topic string `json:"topic"`
	} `json:"cam"`
	User struct {
		User struct {
			ID                 int64  `json:"id"`
			Username           string `json:"username"`
			IsOnline           bool   `json:"isOnline"`
			IsLive             bool   `json:"isLive"`
			Status             string `json:"status"`
			BroadcastGender    string `json:"broadcastGender"`
			PreviewUrlThumbBig string `json:"previewUrlThumbBig"`
			SnapshotTimestamp  int64  `json:"snapshotTimestamp"`
		} `json:"user"`
	} `json:"user"`
}

func mapGender(g string) string {
	switch g {
	case "female":
		return "f"
	case "male":
		return "m"
	case "couple":
		return "c"
	case "trans":
		return "t"
	default:
		return g
	}
}

func (s *StripchatSite) FetchStream(ctx context.Context, req *internal.Req, username string) (*site.StreamInfo, error) {
	apiURL := fmt.Sprintf("https://stripchat.com/api/front/v2/models/username/%s/cam", username)

	body, err := req.Get(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("stripchat: fetch cam: %w", err)
	}

	var resp camResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("stripchat: parse cam response: %w", err)
	}

	u := resp.User.User

	// Build room status string from Stripchat's status field.
	roomStatus := u.Status
	if !u.IsOnline && !u.IsLive {
		if roomStatus == "" {
			roomStatus = site.StatusOffline
		}
	}

	tags := []string{}
	if resp.Cam.Topic != "" {
		for _, word := range strings.Fields(resp.Cam.Topic) {
			if strings.HasPrefix(word, "#") {
				tag := strings.TrimPrefix(word, "#")
				tag = strings.Trim(tag, ".,!?;:")
				if tag != "" {
					tags = append(tags, tag)
				}
			}
		}
	}

	// Stripchat's previewUrlThumbBig may be a cached CDN URL; append the
	// snapshot timestamp to force a fresh image on each API poll, similar
	// to how Chaturbate's thumb.live.mmcdn.com always returns the latest frame.
	thumbURL := u.PreviewUrlThumbBig
	if thumbURL != "" && u.SnapshotTimestamp > 0 {
		if strings.Contains(thumbURL, "?") {
			thumbURL = fmt.Sprintf("%s&t=%d", thumbURL, u.SnapshotTimestamp)
		} else {
			thumbURL = fmt.Sprintf("%s?t=%d", thumbURL, u.SnapshotTimestamp)
		}
	}

	info := &site.StreamInfo{
		RoomStatus:   roomStatus,
		RoomTitle:    resp.Cam.Topic,
		Tags:         tags,
		Gender:       mapGender(u.BroadcastGender),
		LiveThumbURL: thumbURL,
	}

	if !u.IsOnline && !u.IsLive {
		return info, internal.ErrChannelOffline
	}
	if !resp.Cam.IsCamActive {
		return info, internal.ErrChannelOffline
	}
	if roomStatus != "public" {
		return info, internal.ErrChannelOffline
	}

	streamName := resp.Cam.StreamName

	var hlsURL string
	if server, ok := resp.Cam.ViewServers["flashphoner-hls"]; ok && server != "" {
		hlsURL = fmt.Sprintf(
			"https://b-%s.doppiocdn.com/hls/%s/master_%s.m3u8",
			server, streamName, streamName,
		)
	} else {
		hlsURL = fmt.Sprintf(
			"https://edge-hls.doppiocdn.net/hls/%s/master/%s_auto.m3u8?playlistType=lowLatency",
			streamName, streamName,
		)
	}

	info.HLSSource = hlsURL
	return info, nil
}

func (s *StripchatSite) GetRoomStatus(ctx context.Context, req *internal.Req, username string) (string, error) {
	apiURL := fmt.Sprintf("https://stripchat.com/api/front/v2/models/username/%s/cam", username)

	body, err := req.Get(ctx, apiURL)
	if err != nil {
		return "", fmt.Errorf("stripchat: get room status: %w", err)
	}

	var resp camResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return "", fmt.Errorf("stripchat: parse cam response: %w", err)
	}

	status := resp.User.User.Status
	if status == "" {
		if !resp.User.User.IsOnline && !resp.User.User.IsLive {
			return site.StatusOffline, nil
		}
		return "unknown", nil
	}
	return status, nil
}

var _ site.Site = (*StripchatSite)(nil)
