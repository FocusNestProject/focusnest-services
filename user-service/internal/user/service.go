package user

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type service struct {
	repo Repository
}

// NewService creates a new user service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetProfile(ctx context.Context, userID string) (*ProfileResponse, error) {
	var (
		profile  *Profile
		metadata ProfileMetadata
	)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		p, err := s.repo.GetProfile(ctx, userID)
		if err != nil {
			return err
		}
		profile = p
		return nil
	})

	g.Go(func() error {
		m, err := s.repo.GetProfileMetadata(ctx, userID)
		if err != nil {
			return err
		}
		metadata = m
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return buildProfileResponse(profile, metadata), nil
}

func (s *service) UpdateProfile(ctx context.Context, userID string, updates ProfileUpdateInput) (*ProfileResponse, error) {
	var (
		updated  *Profile
		metadata ProfileMetadata
	)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		p, err := s.repo.UpsertProfile(ctx, userID, updates)
		if err != nil {
			return err
		}
		updated = p
		return nil
	})

	g.Go(func() error {
		m, err := s.repo.GetProfileMetadata(ctx, userID)
		if err != nil {
			return err
		}
		metadata = m
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return buildProfileResponse(updated, metadata), nil
}

func defaultProfile(userID string) *Profile {
	return &Profile{UserID: userID}
}

func buildProfileResponse(profile *Profile, metadata ProfileMetadata) *ProfileResponse {
	return &ProfileResponse{
		UserID:          profile.UserID,
		Bio:             profile.Bio,
		Birthdate:       profile.Birthdate,
		ProfileMetadata: metadata,
		CreatedAt:       profile.CreatedAt,
		UpdatedAt:       profile.UpdatedAt,
	}
}
