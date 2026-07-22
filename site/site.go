package site

import (
	"context"

	"github.com/teacat/chaturbate-dvr/internal"
)

// Room status constants from the Chaturbate devportal:
//   https://devportal.cb.dev/wiki/api/$room#roomstatus-string
const (
	StatusPublic            = "public"
	StatusPrivate           = "private"
	StatusHidden            = "hidden"             // Limitcam session in progress
	StatusAway              = "away"               // Away mode after a Private
	StatusOffline           = "offline"
	StatusPasswordProtected = "password protected"  // Password protected room
	StatusGeoBlocked        = "geo_blocked"         // Public but HLS URL empty (region-restricted)
)

// OnlineConfidence represents how sure we are that a channel is live.
type OnlineConfidence int

const (
	ConfidenceConfirmedOffline OnlineConfidence = iota // All APIs confirmed offline
	ConfidenceProbablyOffline                          // Single API says offline
	ConfidenceUncertain                                // APIs disagree or intermittent errors
	ConfidenceProbablyOnline                           // API says public but no HLS (geo-blocked)
	ConfidenceConfirmedOnline                          // API says public + valid HLS URL
)

// IsConsideredLive returns true if the room status means the model is
// actively broadcasting (even if we can't record).
func IsConsideredLive(status string) bool {
	switch status {
	case StatusPublic, StatusPrivate, StatusHidden, StatusGeoBlocked:
		return true
	default:
		return false
	}
}

// IsRecordable returns true if we can actually download the HLS stream.
func IsRecordable(status string) bool {
	return status == StatusPublic
}

// StreamInfo holds the result of fetching a stream for a model.
type StreamInfo struct {
	HLSSource    string
	RoomStatus   string
	RoomTitle    string
	Tags         []string
	NumUsers     int
	Gender       string
	LiveThumbURL string // live-updating thumbnail URL; empty = use site default
}

// Site is the interface that each live cam site must implement.
type Site interface {
	// FetchStream retrieves the HLS stream URL and room metadata.
	// Returns a non-nil StreamInfo even on error so callers can always
	// read RoomStatus.  The error indicates whether a stream URL was
	// obtained; RoomStatus reflects the current state of the room.
	FetchStream(ctx context.Context, req *internal.Req, username string) (*StreamInfo, error)

	// GetRoomStatus returns the room status string (public, private, away, offline, etc.).
	GetRoomStatus(ctx context.Context, req *internal.Req, username string) (string, error)
}
