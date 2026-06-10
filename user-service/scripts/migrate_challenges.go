//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	"github.com/focusnest/user-service/internal/user"
)

// We duplicate the challenge definitions here for the migration script.
func getInitialChallenges() []user.ChallengeDefinition {
	return []user.ChallengeDefinition{
		{
			ID:               "focus_2h_3days",
			Title:            "Fokus 2 jam selama 3 hari",
			Description:      "Fokus minimal 2 jam tanpa distraksi selama 3 hari berturut-turut",
			RewardPoints:     50,
			RuleType:         user.ChallengeRuleDailyMinutesStreak,
			MinMinutesPerDay: 120,
			ConsecutiveDays:  3,
		},
		{
			ID:           "share_recap_3x_weekly",
			Title:        "Bagikan Recap Mingguan",
			Description:  "Bagikan recap hasil fokusmu 3 kali dalam seminggu",
			RewardPoints: 50,
			RuleType:     user.ChallengeRuleWeeklyShares,
			TargetCount:  3,
		},
		{
			ID:           "streak_10_days",
			Title:        "Raih 10 Hari Streak",
			Description:  "Raih 10 hari streak dan unggah recap ke media sosial",
			RewardPoints: 50,
			RuleType:     user.ChallengeRuleStreakMilestone,
			TargetStreak: 10,
		},
		{
			ID:                       "cycles_and_mindfulness",
			Title:                    "Fokus & Mindfulness",
			Description:              "Selesaikan 4 cycle kerja dan lakukan 2 menit mindfulness breathing",
			RewardPoints:             50,
			RuleType:                 user.ChallengeRuleCyclesAndMindfulness,
			TargetCycles:             4,
			TargetMindfulnessMinutes: 2,
		},
	}
}

func main() {
	ctx := context.Background()

	// Assuming project ID is focusnest-470308 based on deploy.yml
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

	challenges := getInitialChallenges()

	fmt.Printf("Migrating %d challenges to Firestore...\n", len(challenges))

	for _, c := range challenges {
		docRef := client.Collection("challenges").Doc(c.ID)
		_, err := docRef.Set(ctx, c)
		if err != nil {
			log.Fatalf("Failed to migrate challenge %s: %v", c.ID, err)
		}
		fmt.Printf("✅ Successfully migrated challenge: %s\n", c.ID)
	}

	fmt.Println("🎉 Migration completed successfully!")
}
