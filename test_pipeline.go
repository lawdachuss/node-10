//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/teacat/chaturbate-dvr/database"
	"github.com/teacat/chaturbate-dvr/entity"
	"github.com/teacat/chaturbate-dvr/server"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("  SUPABASE PIPELINE END-TO-END TEST")
	fmt.Println("========================================\n")

	// Load environment — fall back to hardcoded values for local dev
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_API_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		supabaseURL = "https://rvbuzyljrwsxfxijotdf.supabase.co"
		supabaseKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InJ2YnV6eWxqcndzeGZ4aWpvdGRmIiwicm9sZSI6ImFub24iLCJpYXQiOjE3ODQxODY5NzcsImV4cCI6MjA5OTc2Mjk3N30.h-9UzssKfzZ3fJrgnRjm1VIbAez3rSP3bfN3XjaGZ1g"
		fmt.Println("ℹ️  Using hardcoded dev credentials (set SUPABASE_URL / SUPABASE_API_KEY to override)\n")
	}

	// Wire up the DB client via server.Config (same path as production)
	server.Config = &entity.Config{
		SupabaseURL:    supabaseURL,
		SupabaseAPIKey: supabaseKey,
	}
	client := server.GetDBClient()
	if client == nil {
		fmt.Println("❌ Could not create DB client — aborting")
		os.Exit(1)
	}
	fmt.Println("✓ Database client initialized\n")

	pass := 0
	fail := 0

	// ─── TEST 1: Save a recording (basics) ────────────────────────────────────
	fmt.Println("┌─ TEST 1: SaveRecordingBasics ───────────────────────────────")
	testUsername := "test_pipeline_user"
	testFilename := fmt.Sprintf("%s_%s.mp4", testUsername, time.Now().UTC().Format("2006-01-02_15-04-05"))
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	if err := server.SaveRecordingBasics(testUsername, testFilename, timestamp, "Test Room", []string{"test"}, 42, "female", "1080p", 30, 161495, 12.5); err != nil {
		fmt.Printf("└─ ❌ FAIL: %v\n\n", err)
		fail++
	} else {
		fmt.Printf("└─ ✓ PASS: saved → %s\n\n", testFilename)
		pass++
	}

	// ─── TEST 2: Get the saved recording ─────────────────────────────────────
	fmt.Println("┌─ TEST 2: GetRecording ──────────────────────────────────────")
	savedRec, err := client.GetRecording(testFilename)
	if err != nil {
		fmt.Printf("└─ ❌ FAIL: GetRecording: %v\n\n", err)
		fail++
		fmt.Println("⚠️  Cannot continue without recording ID — aborting remaining tests")
		cleanupRec(client, testFilename)
		printSummary(pass, fail)
		os.Exit(1)
	}
	fmt.Printf("└─ ✓ PASS: recording ID = %s\n\n", savedRec.ID)
	pass++

	// ─── TEST 3: SaveUploadLinks — the UUID=text fix ──────────────────────────
	fmt.Println("┌─ TEST 3: SaveUploadLinks (UUID casting fix) ────────────────")
	uploadErr := client.SaveUploadLinks([]database.UploadLink{
		{RecordingID: savedRec.ID, Host: "StreamTape", URL: "https://streamtape.com/v/test123"},
		{RecordingID: savedRec.ID, Host: "VoeSX", URL: "https://voe.sx/e/test456"},
		{RecordingID: savedRec.ID, Host: "SeekStreaming", URL: "https://seekstreaming.com/v/test789"},
	})
	if uploadErr != nil {
		fmt.Printf("└─ ❌ FAIL: SaveUploadLinks: %v\n", uploadErr)
		fmt.Println("   ► Apply BUGFIXES.sql in Supabase SQL editor then re-run\n")
		fail++
	} else {
		fmt.Println("└─ ✓ PASS: upload links saved\n")
		pass++
	}

	// ─── TEST 4: GetUploadLinks ───────────────────────────────────────────────
	fmt.Println("┌─ TEST 4: GetUploadLinks ────────────────────────────────────")
	links, err := client.GetUploadLinks(savedRec.ID)
	if err != nil {
		fmt.Printf("└─ ❌ FAIL: %v\n\n", err)
		fail++
	} else {
		fmt.Printf("└─ ✓ PASS: retrieved %d link(s)\n", len(links))
		for _, l := range links {
			fmt.Printf("   • %-15s → %s\n", l.Host, l.URL)
		}
		fmt.Println()
		pass++
	}

	// ─── TEST 5: SaveRecordingWithLinks (full production-path pipeline) ────────
	fmt.Println("┌─ TEST 5: SaveRecordingWithLinks (full pipeline) ────────────")
	fullFilename := fmt.Sprintf("%s_full_%s.mp4", testUsername, time.Now().UTC().Format("2006-01-02_15-04-05"))
	if err := server.SaveRecordingWithLinks(
		testUsername, fullFilename, timestamp,
		"Full Pipeline Test Room", []string{"test", "pipeline"}, 99,
		"720p", 60, 17366607, 55.2,
		"female",
		"https://embed.example.com/v/fulltest",
		"https://thumb.example.com/fulltest.jpg",
		"",
		map[string]string{
			"StreamTape":    "https://streamtape.com/v/full123",
			"VoeSX":         "https://voe.sx/e/full456",
			"SeekStreaming": "https://seekstreaming.com/v/full789",
		},
	); err != nil {
		fmt.Printf("└─ ❌ FAIL: SaveRecordingWithLinks: %v\n\n", err)
		fail++
	} else {
		fmt.Println("└─ ✓ PASS: full pipeline recording+links saved\n")
		pass++
		_ = client.DeleteRecording(fullFilename)
	}

	// ─── TEST 6: Idempotent upsert ────────────────────────────────────────────
	fmt.Println("┌─ TEST 6: Idempotent upsert (re-save same links, new URL) ───")
	if err := client.SaveUploadLinks([]database.UploadLink{
		{RecordingID: savedRec.ID, Host: "StreamTape", URL: "https://streamtape.com/v/updated_url"},
	}); err != nil {
		fmt.Printf("└─ ❌ FAIL: idempotent upsert: %v\n\n", err)
		fail++
	} else {
		fmt.Println("└─ ✓ PASS: idempotent upsert (URL updated)\n")
		pass++
	}

	// ─── TEST 7: DeleteUploadLinks ────────────────────────────────────────────
	fmt.Println("┌─ TEST 7: DeleteUploadLinksByRecordingID ────────────────────")
	if err := client.DeleteUploadLinksByRecordingID(savedRec.ID); err != nil {
		fmt.Printf("└─ ❌ FAIL: %v\n\n", err)
		fail++
	} else {
		fmt.Println("└─ ✓ PASS: upload links deleted\n")
		pass++
	}

	// ─── Cleanup ──────────────────────────────────────────────────────────────
	cleanupRec(client, testFilename)
	printSummary(pass, fail)
}

func cleanupRec(client *database.Client, filename string) {
	fmt.Printf("🧹 Deleting test recording %q...\n", filename)
	if err := client.DeleteRecording(filename); err != nil {
		fmt.Printf("   ⚠️  Could not delete: %v\n", err)
	} else {
		fmt.Println("   ✓ Deleted")
	}
	fmt.Println()
}

func printSummary(pass, fail int) {
	fmt.Println("========================================")
	fmt.Printf("  RESULTS: %d passed, %d failed\n", pass, fail)
	fmt.Println("========================================")
	if fail > 0 {
		os.Exit(1)
	}
}
