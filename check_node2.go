//go:build ignore
// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type Node struct {
	NodeID          string `json:"node_id"`
	Hostname        string `json:"hostname"`
	InstanceLabel   string `json:"instance_label"`
	SoftwareVersion string `json:"software_version"`
	Status          string `json:"status"`
	CurrentLoad     int    `json:"current_load"`
	LastHeartbeat   string `json:"last_heartbeat"`
	WebURL          string `json:"web_url"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

func main() {
	supabaseURL := "https://rvbuzyljrwsxfxijotdf.supabase.co"
	apiKey := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InJ2YnV6eWxqcndzeGZ4aWpvdGRmIiwicm9sZSI6ImFub24iLCJpYXQiOjE3ODQxODY5NzcsImV4cCI6MjA5OTc2Mjk3N30.h-9UzssKfzZ3fJrgnRjm1VIbAez3rSP3bfN3XjaGZ1g"

	// Query for all nodes
	url := supabaseURL + "/rest/v1/nodes?order=node_id.asc"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		os.Exit(1)
	}

	req.Header.Set("apikey", apiKey)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != 200 {
		fmt.Printf("HTTP %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var nodes []Node
	if err := json.Unmarshal(body, &nodes); err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	if len(nodes) == 0 {
		fmt.Println("No nodes found in database")
		os.Exit(0)
	}

	fmt.Printf("\n=== All Nodes ===\n")
	for _, node := range nodes {
		fmt.Printf("\nNode ID: %s\n", node.NodeID)
		fmt.Printf("  Hostname: %s\n", node.Hostname)
		fmt.Printf("  Instance Label: %s\n", node.InstanceLabel)
		fmt.Printf("  Status: %s\n", node.Status)
		fmt.Printf("  Current Load: %d\n", node.CurrentLoad)
		fmt.Printf("  Last Heartbeat: %s\n", node.LastHeartbeat)
		fmt.Printf("  Web URL: %s\n", node.WebURL)
		fmt.Printf("  Software Version: %s\n", node.SoftwareVersion)

		if node.WebURL != "" {
			fmt.Printf("  ✓ Dashboard: %s\n", node.WebURL)
			fmt.Printf("  ✓ Logs: %s/logs\n", node.WebURL)
		}
	}
}
