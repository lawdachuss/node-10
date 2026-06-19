package channel

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/teacat/chaturbate-dvr/entity"
	"github.com/teacat/chaturbate-dvr/server"
)

// TestPipelineQueueRestartableAfterStop verifies the queue can process new work
// after Stop() has drained and exited its worker.
//
// Defense-in-depth for a latent footgun: startOnce() used to set started=true
// exactly once and never reset it, while Stop() left stopped=true permanently.
// After a Stop the worker was dead and startOnce() was a no-op, so a later
// EnqueueFile would never be processed (it would fall into the "queue stopped"
// recovery branch and only persist to DB).
//
// Proof of real processing: the pipeline must land in History (a live worker
// ran it), NOT merely be absent from the queue (which the stopped-recovery
// branch also satisfies — that would be a false positive).
func TestPipelineQueueRestartableAfterStop(t *testing.T) {
	oldConfig := server.Config
	defer func() { server.Config = oldConfig }()
	server.Config = &entity.Config{}

	dir := t.TempDir()
	path := filepath.Join(dir, "alice_2025-01-01_12-00-00.mp4")
	if err := os.WriteFile(path, []byte("video"), 0o666); err != nil {
		t.Fatalf("write: %v", err)
	}

	ch := &Channel{
		Config:   &entity.ChannelConfig{Username: "alice"},
		LogCh:    make(chan string, 20),
		UpdateCh: make(chan bool, 1),
	}
	pq := NewPipelineQueue(ch)

	// First lifecycle: enqueue, let it run, then stop.
	pq.EnqueueFile(path)
	pq.Stop()

	// Second lifecycle: the same queue must relaunch its worker and process.
	path2 := filepath.Join(dir, "alice_2025-01-02_13-00-00.mp4")
	if err := os.WriteFile(path2, []byte("video2"), 0o666); err != nil {
		t.Fatalf("write2: %v", err)
	}
	pq.EnqueueFile(path2)

	// A relaunched worker will run the pipeline to completion/failure and push
	// it into History.  Poll History for the second filename.
	deadline := time.Now().Add(15 * time.Second)
	saw := false
	for time.Now().Before(deadline) {
		for _, h := range pq.HistoryEntries() {
			if h.Filename == "alice_2025-01-02_13-00-00.mp4" {
				saw = true
			}
		}
		if saw {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	pq.Stop() // clean up the relaunched worker

	if !saw {
		t.Fatal("second pipeline never reached History after Stop — worker was not restarted")
	}
}
