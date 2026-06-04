package channel

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/teacat/chaturbate-dvr/chaturbate"
	"github.com/teacat/chaturbate-dvr/internal"
	"github.com/teacat/chaturbate-dvr/server"
)

// Monitor starts monitoring the channel for live streams and records them.
func (ch *Channel) Monitor() {
	client := chaturbate.NewClient()
	ch.Info("starting to record `%s`", ch.Config.Username)

	// Create a new context with a cancel function,
	// the CancelFunc will be stored in the channel's CancelFunc field
	// and will be called by `Pause` or `Stop` functions
	ctx, _ := ch.WithCancel(context.Background())

	ch.stateMu.Lock()
	ch.RoomStatus = "offline"
	ch.stateMu.Unlock()

	var err error
	for {
		if err = ctx.Err(); err != nil {
			break
		}

		pipeline := func() error {
			return ch.RecordStream(ctx, client)
		}

		onRetry := func(_ uint, err error) {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return
			}
			switch {
			case errors.Is(err, internal.ErrChannelOffline) || errors.Is(err, internal.ErrPrivateStream):
				ch.stateMu.Lock()
				ch.IsOnline = false
				ch.IsConnecting = false
				ch.RoomStatus = client.LastRoomStatus
				roomStatus := ch.RoomStatus
				ch.stateMu.Unlock()
				ch.Update()
				if client.LastRoomStatus == chaturbate.StatusPublic && errors.Is(err, internal.ErrChannelOffline) {
					ch.Info("channel is live but stream URL unavailable (check cookies); try again in %d min(s)", server.Config.Interval)
				} else {
					ch.Info("channel is %s, try again in %d min(s)", roomStatus, server.Config.Interval)
				}

				// NOTE: no extra Cleanup call here.
				// RecordStream's deferred Cleanup(false) always runs before
				// onRetry is called (defers execute before the function returns
				// to the retry loop). ch.File is therefore always nil at this
				// point. Launching a goroutine here was redundant dead-code and
				// caused a race: if the goroutine was delayed by the scheduler
				// it could fire after the next RecordStream had already opened
				// new files, closing and uploading them prematurely.

			case errors.Is(err, internal.ErrStreamStalled):
				// CDN session expired mid-stream (common with LL-HLS tokens).
				// The stream is still live — do NOT set IsOnline=false.
				// Show "Reconnecting..." in the UI while we fetch a fresh URL.
				// Skip HEAD edge validation to reconnect as fast as possible.
				client.SkipEdgeCheck = true
				ch.SetConnecting(true)
				ch.Info("stream stalled (CDN session expired) — reconnecting in 15s")

			default:
				// Transient network errors (DNS, TCP, etc).
				// Show "Reconnecting..." in the UI while we retry.
				ch.SetConnecting(true)
				ch.Error("on retry: %s: retrying", err.Error())
			}
		}

		customDelay := func(n uint, err error, _ *retry.Config) time.Duration {
			switch {
			case errors.Is(err, internal.ErrStreamStalled):
				// CDN token refresh: quick retry.
				return 15*time.Second + time.Duration(rand.Int63n(16))*time.Second
			case errors.Is(err, internal.ErrChannelOffline) || errors.Is(err, internal.ErrPrivateStream):
				// Truly offline: standard interval with jitter.
				base := time.Duration(server.Config.Interval) * time.Minute
				return base + time.Duration(rand.Int63n(31))*time.Second
			default:
				// Transient errors: exponential backoff 10s→20s→40s→80s→160s, max 300s.
				backoff := time.Duration(min(1<<n, 32)) * 10 * time.Second
				return backoff + time.Duration(rand.Int63n(11))*time.Second
			}
		}

		if err = retry.Do(
			pipeline,
			retry.Context(ctx),
			retry.Attempts(0),
			retry.DelayType(customDelay),
			retry.OnRetry(onRetry),
		); err != nil {
			break
		}
	}

	// Always cleanup when monitor exits, regardless of error
	if err := ch.Cleanup(false); err != nil {
		ch.Error("cleanup on monitor exit: %s", err.Error())
	}

	// Log error if it's not a context cancellation
	if err != nil && !errors.Is(err, context.Canceled) {
		ch.Error("record stream: %s", err.Error())
	}
}

// Update sends an update signal to the channel's update channel.
// This notifies the Server-sent Event to boradcast the channel information to the client.
func (ch *Channel) Update() {
	select {
	case ch.UpdateCh <- true:
	default:
	}
}

const defaultGracePeriod = 3 * time.Minute // how long to wait before finalising a recording on transient errors

// RecordStream records the stream of the channel using the provided client.
// It retrieves the stream information and starts watching the segments.
func (ch *Channel) RecordStream(ctx context.Context, client *chaturbate.Client) error {
	stream, err := client.GetStream(ctx, ch.Config.Username)
	if err != nil {
		return fmt.Errorf("get stream: %w", err)
	}
	playlist, err := stream.GetPlaylist(ctx, ch.Config.Resolution, ch.Config.Framerate)
	if err != nil {
		return fmt.Errorf("get playlist: %w", err)
	}

	// Capture room metadata cached on the client from GetStream.
	ch.RoomTitle = client.LastRoomTitle
	ch.Tags = client.LastTags
	ch.Viewers = client.LastViewers
	ch.Gender = client.LastGender

	// Fallback: if the API returned no tags array, extract hashtags from
	// the room title (e.g. "title #tag1 #tag2").
	if len(ch.Tags) == 0 && ch.RoomTitle != "" {
		ch.Tags = extractHashtags(ch.RoomTitle)
	}

	// Capture actual stream quality from the playlist
	ch.Resolution = fmt.Sprintf("%dp", playlist.Resolution)
	ch.Framerate = playlist.Framerate

	ch.StreamedAt = time.Now().Unix()
	ch.Sequence = 0
	ch.InitSegment = nil
	ch.AudioInitSegment = nil
	ch.HasSeparateAudio = playlist.AudioPlaylistURL != ""
	ch.switchRequested = false
	ch.videoSegmentCount = 0
	ch.audioSegmentCount = 0

	if err := ch.NextFile(); err != nil {
		return fmt.Errorf("next file: %w", err)
	}

	// Ensure file is cleaned up when this function exits in any case
	defer func() {
		if err := ch.Cleanup(false); err != nil {
			ch.Error("cleanup on record stream exit: %s", err.Error())
		}
	}()

	ch.stateMu.Lock()
	ch.RoomStatus = chaturbate.StatusPublic
	ch.stateMu.Unlock()
	ch.UpdateOnlineStatus(true) // after GetPlaylist succeeds

	ch.Info("stream quality - %dp @ %dfps (target: %dp @ %dfps)", playlist.Resolution, playlist.Framerate, ch.Config.Resolution, ch.Config.Framerate)
	if ch.HasSeparateAudio {
		ch.Info("mux: separate audio track detected — will mux audio/video after recording")
	}
	if ch.Viewers > 0 {
		ch.Info("status: %d viewers", ch.Viewers)
	}
	if ch.RoomTitle != "" {
		title := ch.RoomTitle
		if len(title) > 80 {
			title = title[:80] + "…"
		}
		ch.Info("status: room title: %s", title)
	}

	return ch.watchWithGrace(ctx, client, playlist)
}

// watchWithGrace wraps WatchAVSegments with a grace period so that transient
// errors (CDN token expiry, network blips, brief API hiccups) do not finalise
// the recording prematurely.  Instead of returning the error immediately
// (which triggers Cleanup → mux → upload of a short file), the function polls
// the Chaturbate API every 30s for up to defaultGracePeriod.  If the channel
// comes back online within that window, a fresh HLS playlist URL is obtained
// and WatchAVSegments resumes writing to the same file.  Only after the grace
// period expires does the error propagate to the deferred Cleanup.
func (ch *Channel) watchWithGrace(ctx context.Context, client *chaturbate.Client, p *chaturbate.Playlist) error {
	const pollInterval = 30 * time.Second

	origErr := ch.watchLoop(ctx, client, p)
	if origErr == nil {
		return nil
	}
	if errors.Is(origErr, context.Canceled) || errors.Is(origErr, context.DeadlineExceeded) {
		return origErr
	}

	ch.Info("recording: segment fetch interrupted (%s); %s grace period starts", origErr, defaultGracePeriod)

	deadline := time.Now().Add(defaultGracePeriod)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}

		stream, apiErr := client.GetStream(ctx, ch.Config.Username)
		if apiErr != nil {
			ch.Info("recording: API still unavailable (%s); %s remaining", apiErr, time.Until(deadline).Round(time.Second))
			continue
		}

		newPlaylist, apiErr := stream.GetPlaylist(ctx, ch.Config.Resolution, ch.Config.Framerate)
		if apiErr != nil {
			continue
		}

		ch.Info("recording: channel back online — resuming with fresh playlist")

		ch.stateMu.Lock()
		ch.RoomStatus = chaturbate.StatusPublic
		ch.stateMu.Unlock()
		ch.UpdateOnlineStatus(true)

		// Verify the playlist covers the same quality before proceeding.
		ch.Resolution = fmt.Sprintf("%dp", newPlaylist.Resolution)
		ch.Framerate = newPlaylist.Framerate

		// Re-enter WatchAVSegments with the fresh URL.  Init handlers are
		// idempotent — they skip the write when ch.InitSegment is already set.
		loopErr := ch.watchLoop(ctx, client, newPlaylist)
		if loopErr == nil {
			return nil
		}
		if errors.Is(loopErr, context.Canceled) || errors.Is(loopErr, context.DeadlineExceeded) {
			return loopErr
		}

		ch.Info("recording: segment fetch interrupted again during grace period (%s)", loopErr)
	}

	ch.Info("recording: grace period expired — finalizing file")
	return fmt.Errorf("channel offline after grace period: %w", origErr)
}

// watchLoop is a thin wrapper around WatchAVSegments that handles the common
// stall-detection error by obtaining a fresh HLS URL and retrying immediately
// (without entering the full grace period).  Higher-level errors fall through
// to the grace-period loop in watchWithGrace.
func (ch *Channel) watchLoop(ctx context.Context, client *chaturbate.Client, p *chaturbate.Playlist) error {
	err := p.WatchAVSegments(ctx, ch.HandleSegment, ch.HandleInitSegment, ch.HandleAudioSegment, ch.HandleAudioInitSegment, ch.OnPollComplete)
	if err != nil && errors.Is(err, internal.ErrStreamStalled) {
		ch.Info("recording: CDN session expired — fetching fresh playlist URL")
		stream, apiErr := client.GetStream(ctx, ch.Config.Username)
		if apiErr != nil {
			return apiErr
		}
		newPlaylist, apiErr := stream.GetPlaylist(ctx, ch.Config.Resolution, ch.Config.Framerate)
		if apiErr != nil {
			return apiErr
		}
		ch.Resolution = fmt.Sprintf("%dp", newPlaylist.Resolution)
		ch.Framerate = newPlaylist.Framerate
		ch.stateMu.Lock()
		ch.RoomStatus = chaturbate.StatusPublic
		ch.stateMu.Unlock()
		ch.UpdateOnlineStatus(true)
		return ch.watchLoop(ctx, client, newPlaylist)
	}
	return err
}

// HandleInitSegment stores the fMP4 init segment and writes it to the file.
// On subsequent calls (e.g. after a grace-period reconnect with a fresh CDN
// session), the write is skipped because the moov atom is already in the file.
func (ch *Channel) HandleInitSegment(initData []byte) error {
	if ch.InitSegment != nil {
		return nil
	}
	ch.InitSegment = initData

	if ch.File == nil {
		return nil
	}

	n, err := ch.File.Write(initData)
	if err != nil {
		return fmt.Errorf("write init segment: %w", err)
	}
	ch.stateMu.Lock()
	ch.Filesize += n
	ch.stateMu.Unlock()
	return nil
}

// HandleAudioInitSegment stores the fMP4 audio init segment and writes it to the file.
// On subsequent calls (e.g. after a grace-period reconnect) the write is skipped.
func (ch *Channel) HandleAudioInitSegment(initData []byte) error {
	if ch.AudioInitSegment != nil {
		return nil
	}
	ch.AudioInitSegment = initData

	if ch.AudioFile == nil {
		return nil
	}

	n, err := ch.AudioFile.Write(initData)
	if err != nil {
		return fmt.Errorf("write audio init segment: %w", err)
	}
	ch.stateMu.Lock()
	ch.Filesize += n
	ch.stateMu.Unlock()
	return nil
}

// HandleSegment processes and writes segment data to a file.
func (ch *Channel) HandleSegment(b []byte, duration float64) error {
	if ch.Config.IsPaused.Load() {
		return retry.Unrecoverable(internal.ErrPaused)
	}

	if ch.File == nil {
		return fmt.Errorf("HandleSegment: ch.File is nil")
	}

	n, err := ch.File.Write(b)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	ch.stateMu.Lock()
	ch.Filesize += n
	ch.Duration += duration
	ch.videoSegmentCount++
	dur := ch.Duration
	fs := ch.Filesize
	vSegCount := ch.videoSegmentCount
	aSegCount := ch.audioSegmentCount
	ch.stateMu.Unlock()

	// Log A/V sync status for dual-stream recordings
	if ch.HasSeparateAudio {
		segDiff := vSegCount - aSegCount
		if segDiff != 0 {
			ch.Info("duration: %s, filesize: %s [v:%d a:%d Δ%+d]", internal.FormatDuration(dur), internal.FormatFilesize(fs), vSegCount, aSegCount, segDiff)
		} else {
			ch.Info("duration: %s, filesize: %s [v:%d a:%d synced]", internal.FormatDuration(dur), internal.FormatFilesize(fs), vSegCount, aSegCount)
		}
	} else {
		ch.Info("duration: %s, filesize: %s", internal.FormatDuration(dur), internal.FormatFilesize(fs))
	}

	// Send an SSE update to update the view
	ch.Update()

	if !ch.ShouldSwitchFile() {
		return nil
	}

	// For LL-HLS streams with separate audio, defer the rotation until the
	// current poll cycle finishes so the paired audio segments land in the
	// same file as the video ones. Single-stream recordings have no pairing
	// risk, and deferring would let processMediaPlaylist keep appending a
	// backlog of catch-up segments past the MaxFilesize/MaxDuration limit.
	if ch.HasSeparateAudio {
		ch.switchRequested = true
		return nil
	}

	if err := ch.NextFile(); err != nil {
		return fmt.Errorf("next file: %w", err)
	}
	ch.Info("max filesize or duration exceeded, new file created: %s", ch.File.Name())
	return nil
}

// OnPollComplete performs any file rotation requested during the poll cycle.
// Called by WatchAVSegments after both video and audio playlists have been
// processed, guaranteeing that rotation never splits an A/V pair.
func (ch *Channel) OnPollComplete() error {
	if !ch.switchRequested {
		return nil
	}
	ch.switchRequested = false
	if err := ch.NextFile(); err != nil {
		return fmt.Errorf("next file: %w", err)
	}
	ch.Info("max filesize or duration exceeded, new file created: %s", ch.File.Name())
	return nil
}

// HandleAudioSegment processes and writes audio segment data to a sidecar file.
func (ch *Channel) HandleAudioSegment(b []byte, duration float64) error {
	if ch.AudioFile == nil {
		return nil
	}
	if ch.Config.IsPaused.Load() {
		return retry.Unrecoverable(internal.ErrPaused)
	}

	if _, err := ch.AudioFile.Write(b); err != nil {
		return fmt.Errorf("write audio file: %w", err)
	}

	ch.stateMu.Lock()
	ch.audioSegmentCount++
	ch.stateMu.Unlock()

	return nil
}

// extractHashtags pulls #word tokens out of a Chaturbate room title and
// returns them as a clean tag list.  Used as a fallback when the API
// returns an empty tags array.
func extractHashtags(title string) []string {
	var tags []string
	for _, word := range strings.Fields(title) {
		if !strings.HasPrefix(word, "#") {
			continue
		}
		tag := strings.TrimPrefix(word, "#")
		tag = strings.Trim(tag, ".,!?;:")
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}
