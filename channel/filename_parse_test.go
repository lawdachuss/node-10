package channel

import "testing"

// TestFilenameParsing covers extractUsernameFromFilename and
// extractTimestampFromFilename for the recorder's filename format:
//
//	"username_YYYY-MM-DD_HH-MM-SS.ext"
//
// (default pattern in entity/entity.go).  Both functions must locate the
// *real* date separator and must not be fooled by a username that itself
// contains "_20" (regression: previously the first "_20" was matched,
// breaking parsing for usernames such as "alice_20_fan").
func TestFilenameParsing(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantUser string
		wantTS   string
	}{
		{
			name:     "standard",
			input:    "alice_2025-01-01_12-00-00.mp4",
			wantUser: "alice",
			wantTS:   "2025-01-01T12:00:00Z",
		},
		{
			name:     "username with underscore",
			input:    "alice_bob_2025-01-01_12-00-00.mp4",
			wantUser: "alice_bob",
			wantTS:   "2025-01-01T12:00:00Z",
		},
		{
			// Regression: username containing "_20" must not be mis-split.
			name:     "username containing _20 but not a date",
			input:    "alice_20_fan_2025-01-01_12-00-00.mp4",
			wantUser: "alice_20_fan",
			wantTS:   "2025-01-01T12:00:00Z",
		},
		{
			// The merge system prepends "merged-".
			name:     "merged prefix",
			input:    "merged-bob_2024-12-31_23-59-59.mkv",
			wantUser: "bob",
			wantTS:   "2024-12-31T23:59:59Z",
		},
		{
			// A merged file from two segments of the same user becomes
			// "merged-user-user_..." and must collapse back to one user.
			name:     "merged dedup username",
			input:    "merged-awesome-sona-awesome-sona_2025-06-19_08-30-00.mp4",
			wantUser: "awesome-sona",
			wantTS:   "2025-06-19T08:30:00Z",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if gotUser := extractUsernameFromFilename(c.input); gotUser != c.wantUser {
				t.Errorf("extractUsernameFromFilename(%q): got %q, want %q", c.input, gotUser, c.wantUser)
			}
			if gotTS := extractTimestampFromFilename(c.input); gotTS != c.wantTS {
				t.Errorf("extractTimestampFromFilename(%q): got %q, want %q", c.input, gotTS, c.wantTS)
			}
		})
	}
}
