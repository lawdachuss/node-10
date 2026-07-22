package coordinator

import (
	"context"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/teacat/chaturbate-dvr/internal"
	"github.com/teacat/chaturbate-dvr/server"
)

// StartLiveCheckLoop periodically checks which channels are live and updates
// the is_live flag in channel_assignments. Runs every 120 seconds.
// Requires LiveCheck to be set; if nil, this is a no-op.
func (c *Coordinator) StartLiveCheckLoop(ctx context.Context) {
	if !c.IsPooled() || c.Client == nil || c.LiveCheck == nil {
		return
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		// Random initial delay (0-30s) to prevent thundering herd
		time.Sleep(time.Duration(rand.Intn(30)) * time.Second)

		ticker := time.NewTicker(120 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			case <-ticker.C:
				c.runSafe("live-check", c.runLiveCheck)
			}
		}
	}()
}

// runLiveCheck checks all channels in the pool and updates their is_live status.
// Uses a two-phase approach:
//   Phase 1: Bulk affiliate API check (single call covers ALL channels) —
//            models found in the affiliate list are immediately marked live.
//   Phase 2: Per-channel IsLive fallback for channels NOT in the affiliate list
//            (catches recently-online channels the affiliate API might have missed).
//
// Reads directly from channel_assignments (the source of truth in pooled mode).
// Uses a 2-minute timeout to prevent a single stuck API call from hanging
// the goroutine indefinitely. Skips entirely when draining.
func (c *Coordinator) runLiveCheck() {
	if c.LiveCheck == nil {
		return
	}

	c.mu.Lock()
	if c.draining {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	assignments, err := c.Client.GetAllAssignments()
	if err != nil || len(assignments) == 0 {
		return
	}

	// ── Phase 1: Bulk affiliate API check ──
	// Fetch ALL online models in one call. Channels in this list are
	// definitively live — no per-channel check needed.
	affiliateLive := make(map[string]bool, len(assignments))
	if wm := server.Config.AffiliateWM; wm != "" {
		models, err := internal.FetchAffiliateOnlineModels(ctx, wm)
		if err == nil {
			for _, ca := range assignments {
				if _, found := models[strings.ToLower(ca.Username)]; found {
					affiliateLive[ca.Username] = true
				}
			}
			log.Printf("[coordinator] affiliate: %d/%d channels live", len(affiliateLive), len(assignments))
		}
	}

	// ── Phase 2: Per-channel fallback ──
	// For channels NOT confirmed live by the affiliate API, do a full
	// per-channel IsLive check (POST→GET cascade with retries).
	var liveUsernames []string
	for _, ca := range assignments {
		isLive := affiliateLive[ca.Username]

		if !isLive {
			// Not found in affiliate list — do a per-channel check.
			// The affiliate API is authoritative for offline, but we still want
			// to catch models that went live between affiliate API calls.
			isLive = c.LiveCheck.IsLive(ctx, ca.Site, ca.Username)
		}

		if isLive {
			liveUsernames = append(liveUsernames, ca.Username)
			if ca.AssignedNode == c.NodeID {
				if err := c.Client.MarkChannelRecording(ca.Username, ca.Site); err != nil {
					log.Printf("[coordinator] live check: mark recording error for %s: %v", ca.Username, err)
				}
			}
		} else if ca.AssignedNode == c.NodeID && ca.Status == "recording" {
			if err := c.Client.SetChannelStatus(ca.Username, ca.Site, "offline"); err != nil {
				log.Printf("[coordinator] live check: set offline error for %s: %v", ca.Username, err)
			}
		}
	}

	// Bulk-update is_live flags
	if len(liveUsernames) > 0 {
		if err := c.Client.SetChannelsLive(liveUsernames); err != nil {
			log.Printf("[coordinator] live check: set live error: %v", err)
		}
		if err := c.Client.SetChannelsNotLive(liveUsernames); err != nil {
			log.Printf("[coordinator] live check: set not live error: %v", err)
		}
	} else {
		if err := c.Client.SetChannelsNotLive([]string{}); err != nil {
			log.Printf("[coordinator] live check: set all not live error: %v", err)
		}
	}
}
