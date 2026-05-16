package internal

import "testing"

func TestCSRFFromCookies(t *testing.T) {
	t.Parallel()
	got := CSRFFromCookies("cf_clearance=abc; csrftoken=real-token; sessionid=x")
	if got != "real-token" {
		t.Fatalf("CSRFFromCookies() = %q, want real-token", got)
	}
}

func TestFormatCookieHeaderSingleCSRFToken(t *testing.T) {
	t.Parallel()
	header := FormatCookieHeader("cf_clearance=abc; csrftoken=old", "new-token")
	if CSRFFromCookies(header) != "new-token" {
		t.Fatalf("csrftoken in header = %q, want new-token", CSRFFromCookies(header))
	}
	if stringsCount(header, "csrftoken=") != 1 {
		t.Fatalf("expected one csrftoken, got header: %s", header)
	}
}

func stringsCount(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}
