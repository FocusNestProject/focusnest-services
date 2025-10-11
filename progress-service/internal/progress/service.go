package progress

import (
	"time"
)

type service struct {
	repo Repository
}

// NewService creates a new progress service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetProgress(userID string, startDate, endDate time.Time) (*ProgressStats, error) {
	return s.repo.GetProgressStats(userID, startDate, endDate)
}
