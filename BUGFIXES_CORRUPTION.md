# Video Corruption & Dropped Files - Bug Analysis & Fixes

## Problem Summary

Files are being **DROPPED PERMANENTLY** when they encounter corruption issues, instead of being quarantined for manual review or automatic retry. This results in **DATA LOSS**.

## Root Causes

### 1. **Normalize Failure → File Dropped**
**Location:** `channel/channel_file.go:normalizeFMP4Timestamps()`
- When ffmpeg fails to normalize timestamps (exit code 0xbebbb1b7), the function returns an error
- The error is logged as a warning, but the file continues to `handleMinDurationAndMerge()`
- If probe then fails, the file is **deleted immediately**

### 2. **Probe Failure → File Dropped Without Quarantine**
**Location:** `channel/channel_file.go:handleMinDurationAndMerge()` line ~1533
```go
// Cannot defer and must not upload a sub-threshold/corrupt file, so drop it.
ch.Error("min-duration: probe failed and cannot defer %s — dropping (no upload)", filepath.Base(videoPath))
os.Remove(videoPath)
return true
```

**Problem:** When probe fails AND the pending directory can't be created, the file is deleted immediately.
- No quarantine directory
- No manual review option
- Permanent data loss

### 3. **Exit Code 0xbebbb1b7**
This is a Windows-specific ffmpeg crash code indicating:
- Memory access violation
- Corrupted input file
- Out of memory
- Driver/codec issue

## Bugs Identified

### Bug #1: No Quarantine for Corrupted Files
**Severity:** CRITICAL - Data Loss
- Files that fail normalization AND probe are deleted
- No way to recover or manually inspect

### Bug #2: No Retry Mechanism for Transient Failures
**Severity:** HIGH
- ffmpeg can fail due to temporary issues (low memory, CPU throttling)
- Files are dropped on first failure, not retried

### Bug #3: Pending Directory Failure Handling
**Severity:** MEDIUM
- If pending dir can't be created, file is dropped
- Should fallback to a quarantine location

### Bug #4: Generic Error Messages
**Severity:** LOW
- "exit status 0xbebbb1b7" is not user-friendly
- No guidance on what went wrong or how to fix

## Proposed Fixes

### Fix #1: Add Quarantine Directory
Create a `quarantine/` directory for files that can't be processed:

```go
// channel/channel_file.go

func (ch *Channel) quarantineFile(videoPath string, reason string) error {
	quarantineDir := filepath.Join(server.Config.VideosDir, "quarantine", ch.Config.Username)
	if err := os.MkdirAll(quarantineDir, 0777); err != nil {
		return fmt.Errorf("create quarantine dir: %w", err)
	}
	
	base := filepath.Base(videoPath)
	timestamp := time.Now().Format("20060102_150405")
	quarantinePath := filepath.Join(quarantineDir, fmt.Sprintf("%s_%s", timestamp, base))
	
	// Write a .txt file with the reason
	reasonFile := quarantinePath + ".reason.txt"
	os.WriteFile(reasonFile, []byte(reason), 0644)
	
	// Move the file to quarantine
	if err := os.Rename(videoPath, quarantinePath); err != nil {
		return fmt.Errorf("move to quarantine: %w", err)
	}
	
	ch.Warn("quarantine: moved %s to quarantine (%s)", base, reason)
	return nil
}
```

### Fix #2: Replace File Deletion with Quarantine
**Location:** `channel/channel_file.go` line ~1533

**Before:**
```go
ch.Error("min-duration: probe failed and cannot defer %s — dropping (no upload)", filepath.Base(videoPath))
os.Remove(videoPath)
return true
```

**After:**
```go
ch.Error("min-duration: probe failed for %s — moving to quarantine", filepath.Base(videoPath))
if qErr := ch.quarantineFile(videoPath, fmt.Sprintf("Probe failed: %v. Could not defer to pending.", err)); qErr != nil {
	ch.Error("min-duration: quarantine failed: %v — file will remain in place", qErr)
	// Don't delete - leave for manual inspection
} else {
	// Successfully quarantined
}
return true
```

### Fix #3: Add Retry Logic for Normalize
**Location:** `channel/channel_file.go:normalizeFMP4Timestamps()`

Add retry with exponential backoff:
```go
func normalizeFMP4TimestampsWithRetry(videoPath string, warn func(string), maxRetries int) (string, error) {
	var lastErr error
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		result, err := normalizeFMP4Timestamps(videoPath, warn)
		if err == nil {
			return result, nil
		}
		
		lastErr = err
		
		// Check if this is a transient error
		if isTransientFFmpegError(err) && attempt < maxRetries {
			backoff := time.Duration(attempt) * 5 * time.Second
			if warn != nil {
				warn(fmt.Sprintf("normalize attempt %d/%d failed (transient), retrying in %s: %v", 
					attempt, maxRetries, backoff, err))
			}
			time.Sleep(backoff)
			continue
		}
		
		// Permanent error - don't retry
		break
	}
	
	return videoPath, fmt.Errorf("normalize failed after %d attempts: %w", maxRetries, lastErr)
}

func isTransientFFmpegError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Windows exit codes that might be transient
	transientCodes := []string{
		"0xbebbb1b7",  // Memory access violation (could be OOM)
		"exit status 1", // Generic failure (might be temporary)
	}
	for _, code := range transientCodes {
		if strings.Contains(errStr, code) {
			return true
		}
	}
	return false
}
```

### Fix #4: Better Error Messages

Add detailed error context:
```go
func analyzeFFmpegError(err error, videoPath string) string {
	if err == nil {
		return ""
	}
	
	errStr := err.Error()
	
	// Windows-specific codes
	if strings.Contains(errStr, "0xbebbb1b7") {
		return "FFmpeg crashed (memory access violation). Possible causes:\n" +
			"  - Corrupted input file\n" +
			"  - Out of memory\n" +
			"  - Graphics driver issue\n" +
			"  - Codec incompatibility"
	}
	
	// Check file size
	if stat, _ := os.Stat(videoPath); stat != nil {
		if stat.Size() == 0 {
			return "File is empty (0 bytes)"
		}
		if stat.Size() < 1024 {
			return fmt.Sprintf("File is suspiciously small (%d bytes)", stat.Size())
		}
	}
	
	return fmt.Sprintf("FFmpeg error: %v", err)
}
```

## Implementation Priority

1. **CRITICAL (Do First):** Fix #1 & #2 - Add quarantine directory and stop deleting files
2. **HIGH:** Fix #3 - Add retry logic for transient failures  
3. **MEDIUM:** Fix #4 - Better error messages

## Testing After Fixes

1. Create a corrupted MP4 file:
   ```bash
   # Create a file with invalid headers
   echo "not a video" > test_corrupt.mp4
   ```

2. Place it in the videos directory and trigger processing

3. Verify it goes to `quarantine/` instead of being deleted

4. Check that a `.reason.txt` file exists explaining why

## Configuration

Add to config:
```go
type Config struct {
	// ...existing fields...
	
	// QuarantineEnabled enables moving corrupted files to quarantine instead of deleting
	QuarantineEnabled bool `json:"quarantine_enabled"`
	
	// NormalizeMaxRetries is how many times to retry timestamp normalization
	NormalizeMaxRetries int `json:"normalize_max_retries"`
}
```

Default values:
```go
QuarantineEnabled: true,     // Always on for safety
NormalizeMaxRetries: 3,      // Try 3 times before giving up
```

## Recovery Process

After applying fixes, to recover existing quarantined files:

1. List files in `quarantine/`
2. For each file, read the `.reason.txt`
3. If reason was transient (e.g., temporary OOM):
   - Move back to videos dir
   - Let pipeline retry
4. If reason indicates permanent corruption:
   - Keep in quarantine
   - Consider re-downloading source if available

## Summary

**Current behavior:** Files are **DELETED** on corruption → **DATA LOSS**
**Fixed behavior:** Files are **QUARANTINED** for review → **NO DATA LOSS**

This is a critical fix that should be applied IMMEDIATELY to prevent further data loss.
