//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {
	supabaseURL := "https://rvbuzyljrwsxfxijotdf.supabase.co"
	// Use service role key for admin operations
	apiKey := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InJ2YnV6eWxqcndzeGZ4aWpvdGRmIiwicm9sZSI6InNlcnZpY2Vfcm9sZSIsImlhdCI6MTc4NDE4Njk3NywiZXhwIjoyMDk5NzYyOTc3fQ.c_A3PFREoH1-8XAjUBRHM_p-IJid4yPn4h0mPQh2BtU"

	// Read the SQL file
	sqlBytes, err := os.ReadFile("BUGFIXES.sql")
	if err != nil {
		fmt.Printf("Error reading BUGFIXES.sql: %v\n", err)
		fmt.Println("\nPlease make sure BUGFIXES.sql exists in the current directory")
		os.Exit(1)
	}

	sqlContent := string(sqlBytes)

	// Split into individual statements (simple split by $$;)
	statements := strings.Split(sqlContent, "$$;")

	fmt.Println("========================================")
	fmt.Println("  APPLYING DATABASE BUG FIXES")
	fmt.Println("========================================\n")

	successCount := 0
	_ = successCount // unused for now

	for i, stmt := range statements {
		// Skip empty statements and comments-only blocks
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}

		// Restore the $$ delimiter if this is a function
		if strings.Contains(stmt, "BEGIN") || strings.Contains(stmt, "RETURNS") {
			stmt = stmt + "$$;"
		}

		// Skip pure comment blocks
		lines := strings.Split(stmt, "\n")
		hasCode := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "--") {
				hasCode = true
				break
			}
		}
		if !hasCode {
			continue
		}

		fmt.Printf("[%d] Executing statement...\n", i+1)

		// Execute via PostgREST RPC (if it's a SELECT or DO block) or direct SQL
		// For Supabase, we need to use the SQL Editor API or RPC
		// Since we can't execute arbitrary SQL via REST API, we'll print instructions instead

		fmt.Printf("    Statement preview: %s...\n", truncate(stmt, 100))
		successCount++
	}

	fmt.Println("\n========================================")
	fmt.Println("  IMPORTANT: MANUAL STEPS REQUIRED")
	fmt.Println("========================================")
	fmt.Println("\nThe BUGFIXES.sql file has been created with all necessary fixes.")
	fmt.Println("\nTo apply these fixes:")
	fmt.Println("1. Go to https://supabase.com/dashboard/project/rvbuzyljrwsxfxijotdf")
	fmt.Println("2. Click on 'SQL Editor' in the left sidebar")
	fmt.Println("3. Click 'New query'")
	fmt.Println("4. Copy the contents of BUGFIXES.sql")
	fmt.Println("5. Paste into the SQL editor")
	fmt.Println("6. Click 'Run' to execute")
	fmt.Println("\nAlternatively, apply section by section if you encounter any errors.")
	fmt.Println("\n========================================")
	fmt.Println("  TESTING CURRENT STATE")
	fmt.Println("========================================\n")

	// Test 1: Check for dead tunnels
	fmt.Println("[TEST 1] Checking for dead tunnels...")
	checkDeadTunnels(supabaseURL, apiKey)

	// Test 2: Check upload_links constraint
	fmt.Println("\n[TEST 2] Checking upload_links constraint...")
	checkUploadLinksConstraint(supabaseURL, apiKey)

	// Test 3: List active tunnels
	fmt.Println("\n[TEST 3] Listing active tunnels...")
	listActiveTunnels(supabaseURL, apiKey)
}

func checkDeadTunnels(supabaseURL, apiKey string) {
	url := supabaseURL + "/rest/v1/tunnels?is_active=eq.true&select=id,url,created_at,instance_id"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("apikey", apiKey)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  ❌ Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		fmt.Printf("  ❌ HTTP %d: %s\n", resp.StatusCode, string(body))
		return
	}

	// Count tunnels
	tunnelCount := strings.Count(string(body), "\"id\":")
	fmt.Printf("  ℹ️  Found %d active tunnel(s)\n", tunnelCount)

	if tunnelCount > 10 {
		fmt.Println("  ⚠️  Warning: Many active tunnels found - some may be dead")
		fmt.Println("     After applying BUGFIXES.sql, old tunnels will auto-expire")
	}
}

func checkUploadLinksConstraint(supabaseURL, apiKey string) {
	// We can't easily query information_schema via PostgREST
	// so we'll just note what to check
	fmt.Println("  ℹ️  After applying fixes, verify the constraint with:")
	fmt.Println("     SELECT constraint_name FROM information_schema.table_constraints")
	fmt.Println("     WHERE table_name = 'upload_links' AND constraint_type = 'UNIQUE';")
	fmt.Println("  ℹ️  Expected result: upload_links_recording_host_unique")
}

func listActiveTunnels(supabaseURL, apiKey string) {
	url := supabaseURL + "/rest/v1/tunnels?is_active=eq.true&order=created_at.desc&limit=10"

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("apikey", apiKey)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("  ❌ Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		fmt.Printf("  ❌ HTTP %d: %s\n", resp.StatusCode, string(body))
		return
	}

	fmt.Printf("  ℹ️  Recent active tunnels:\n%s\n", indentJSON(string(body)))
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func indentJSON(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		if line != "" {
			result = append(result, "     "+line)
		}
	}
	return strings.Join(result, "\n")
}
