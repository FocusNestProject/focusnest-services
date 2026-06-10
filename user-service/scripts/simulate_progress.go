//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/firestore"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run simulate_progress.go <USER_ID>")
	}
	userID := os.Args[1]

	ctx := context.Background()
	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		projectID = "focusnest-470308"
	}
	databaseID := "focusnest-prod"

	client, err := firestore.NewClientWithDatabase(ctx, projectID, databaseID)
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer client.Close()

	now := time.Now().UTC()
	loc, _ := time.LoadLocation("Asia/Jakarta")
	today := time.Now().In(loc)

	fmt.Printf("Simulating progress for user: %s\n", userID)

	// 1. Simulate Daily Minutes Streak (3 days of 2 hours)
	fmt.Println("⏳ Simulating 3 days of focus (2h each)...")
	for i := 0; i < 3; i++ {
		date := today.AddDate(0, 0, -i)
		// Truncate to start of day for summary
		dateKey := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
		
		_, _, err := client.Collection("daily_summaries").Add(ctx, map[string]interface{}{
			"user_id":    userID,
			"date":       dateKey,
			"total_time": 130, // 130 minutes ( > 120 target)
			"sessions":   2,
			"created_at": now,
			"updated_at": now,
		})
		if err != nil {
			log.Printf("Failed to create summary for day %d: %v", i, err)
		}
	}

	// 2. Simulate Weekly Shares (3 shares)
	fmt.Println("📤 Simulating 3 shares...")
	for i := 0; i < 3; i++ {
		_, _, err := client.Collection("profiles").Doc(userID).Collection("shares").Add(ctx, map[string]interface{}{
			"share_type": "recap",
			"shared_at":  now,
		})
		if err != nil {
			log.Printf("Failed to create share %d: %v", i, err)
		}
	}

	// 3. Simulate Cycles & Mindfulness (4 cycles + 2 mins)
	fmt.Println("🧘 Simulating today's cycles and mindfulness...")
	// Cycles (in productivities collection)
	_, _, err = client.Collection("users").Doc(userID).Collection("productivities").Add(ctx, map[string]interface{}{
		"user_id":    userID,
		"num_cycle":  4,
		"start_time": now,
		"anchor":     now,
		"deleted":    false,
	})
	// Mindfulness
	_, _, err = client.Collection("profiles").Doc(userID).Collection("mindfulness").Add(ctx, map[string]interface{}{
		"minutes":      2,
		"completed_at": now,
	})

	fmt.Println("✅ Simulation complete! Now open the app and go to Challenges tab to see them completed.")
}
