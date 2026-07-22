package site

import (
	"context"
	"fmt"

	"github.com/teacat/chaturbate-dvr/chaturbate"
	"github.com/teacat/chaturbate-dvr/internal"
)

// ChaturbateSite adapts the chaturbate package to the Site interface.
type ChaturbateSite struct{}

func NewChaturbateSite() *ChaturbateSite {
	return &ChaturbateSite{}
}

func (s *ChaturbateSite) FetchStream(ctx context.Context, req *internal.Req, username string) (*StreamInfo, error) {
	var roomInfo chaturbate.APIResponse
	stream, roomStatus, err := chaturbate.FetchStream(ctx, req, username, &roomInfo)
	si := &StreamInfo{
		RoomStatus:   roomStatus,
		RoomTitle:    roomInfo.RoomTitle,
		Tags:         roomInfo.Tags,
		NumUsers:     roomInfo.NumUsers,
		Gender:       roomInfo.BroadcasterGender,
		LiveThumbURL: fmt.Sprintf("https://thumb.live.mmcdn.com/ri/%s.jpg", username),
	}
	if err != nil {
		return si, err
	}
	if stream == nil {
		return si, fmt.Errorf("get stream: %w", internal.ErrChannelOffline)
	}
	si.HLSSource = stream.HLSSource
	return si, nil
}

func (s *ChaturbateSite) GetRoomStatus(ctx context.Context, req *internal.Req, username string) (string, error) {
	// Use the advanced multi-tier detection that handles POST→GET cascade,
	// cross-verification, geo-blocked detection, and hidden/password states.
	status, err := chaturbate.GetRoomStatusAdvanced(ctx, req, username)
	if err != nil {
		return "", err
	}

	// Map chaturbate constants to site constants
	switch status {
	case chaturbate.StatusPublic:
		return StatusPublic, nil
	case chaturbate.StatusPrivate:
		return StatusPrivate, nil
	case chaturbate.StatusHidden:
		return StatusHidden, nil
	case chaturbate.StatusAway:
		return StatusAway, nil
	case chaturbate.StatusOffline:
		return StatusOffline, nil
	case chaturbate.StatusPasswordProtected:
		return StatusPasswordProtected, nil
	case chaturbate.StatusGeoBlocked:
		return StatusGeoBlocked, nil
	default:
		return status, nil // pass through unknown status directly
	}
}
