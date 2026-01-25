package user

// challengeDefinitions is the canonical list of available challenges.
// Keep this stable because clients may store challenge IDs.
func challengeDefinitions() []ChallengeDefinition {
	return []ChallengeDefinition{
		{
			ID:               "focus_2h_3days",
			Title:            "Fokus 2 jam selama 3 hari",
			Description:      "Fokus minimal 2 jam sehari selama 3 hari berturut-turut",
			RewardPoints:     50,
			RuleType:         ChallengeRuleDailyMinutesStreak,
			MinMinutesPerDay: 120,
			ConsecutiveDays:  3,
		},
	}
}

// badgesForPoints returns every badge unlocked for the given points.
func badgesForPoints(points int) []Badge {
	// Feel free to tweak thresholds later; IDs should remain stable.
	thresholds := []Badge{
		{ID: "bronze", Label: "Bronze", MinPts: 100},
		{ID: "silver", Label: "Silver", MinPts: 250},
		{ID: "gold", Label: "Gold", MinPts: 500},
		{ID: "diamond", Label: "Diamond", MinPts: 1000},
	}

	var unlocked []Badge
	for _, b := range thresholds {
		if points >= b.MinPts {
			unlocked = append(unlocked, b)
		}
	}
	return unlocked
}

