package channel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teacat/chaturbate-dvr/entity"
	"github.com/teacat/chaturbate-dvr/server"
)

func TestUploadTrackerNormalizesPaths(t *testing.T) {
	path := filepath.Join("videos", "..", "videos", "tracker-test.mp4")
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	MarkUploadInFlight(path)
	t.Cleanup(func() { MarkUploadDone(abs) })

	if !IsUploadInFlight(abs) {
		t.Fatal("absolute path was not recognized as in-flight")
	}

	MarkUploadDone(abs)
	if IsUploadInFlight(path) {
		t.Fatal("path remained in-flight after MarkUploadDone")
	}
}

func TestPipelineCleanupKeepsPartialUploads(t *testing.T) {
	oldConfig := server.Config
	defer func() { server.Config = oldConfig }()
	server.Config = &entity.Config{
		DeleteLocalAfterUpload: true,
		PixelDrainToken:        "configured",
	}

	dir := t.TempDir()
	filePath := filepath.Join(dir, "partial.mp4")
	if err := os.WriteFile(filePath, []byte("video"), 0o666); err != nil {
		t.Fatalf("write video: %v", err)
	}

	ch := &Channel{
		Config:   &entity.ChannelConfig{Username: "tester"},
		LogCh:    make(chan string, 20),
		UpdateCh: make(chan bool, 1),
	}
	p := &Pipeline{
		FilePath: filePath,
		Filename: filepath.Base(filePath),
		Links:    map[string]string{"GoFile": "https://gofile.example/video"},
	}

	if err := p.stageCleanup(ch); err != nil {
		t.Fatalf("stageCleanup partial: %v", err)
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("partial upload file was removed: %v", err)
	}

	p.Links["PixelDrain"] = "https://pixeldrain.example/video"
	if err := p.stageCleanup(ch); err != nil {
		t.Fatalf("stageCleanup complete: %v", err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("complete upload file still exists or stat failed: %v", err)
	}
}
