package channel

import "sync"

var (
	pendingUploadsMu sync.Mutex
	pendingUploads   = make(map[string]struct{})
)

// MarkUploadInFlight records that a file is currently being uploaded by the
// channel system.  The watcher calls IsUploadInFlight before processing a
// file and skips it when this returns true, preventing duplicate uploads
// and the "file not found" race when DeleteLocalAfterUpload fires.
func MarkUploadInFlight(filePath string) {
	pendingUploadsMu.Lock()
	pendingUploads[filePath] = struct{}{}
	pendingUploadsMu.Unlock()
}

// MarkUploadDone removes a file from the in-flight set.  Called via defer in
// upload goroutines so the set is always cleaned up.
func MarkUploadDone(filePath string) {
	pendingUploadsMu.Lock()
	delete(pendingUploads, filePath)
	pendingUploadsMu.Unlock()
}

// IsUploadInFlight returns true if the file is currently being uploaded.
func IsUploadInFlight(filePath string) bool {
	pendingUploadsMu.Lock()
	_, ok := pendingUploads[filePath]
	pendingUploadsMu.Unlock()
	return ok
}
