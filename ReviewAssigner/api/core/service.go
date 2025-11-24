package core

import (
	"context"
	"log/slog"
)

type Service struct {
	log *slog.Logger
	db  DB
}

func NewService(log *slog.Logger, db DB) (*Service, error) {
	return &Service{
		log: log,
		db:  db}, nil
}

func (s *Service) CreateTeam(ctx context.Context, team Team) (Team, error) {
	s.log.Info("create team", "team_name", team.TeamName)

	err := s.db.AddTeam(ctx, team)
	if err != nil {
		return Team{}, err
	}

	return team, nil
}

func (s *Service) GetTeam(ctx context.Context, teamName string) (Team, error) {
	s.log.Info("get team", "team_name", teamName)

	team, err := s.db.GetTeam(ctx, teamName)
	if err != nil {
		return Team{}, err
	}

	return team, nil

}

func (s *Service) IsActive(ctx context.Context, userId string, userStatus bool) (User, error) {
	s.log.Info("setting active status for user", "user_id", userId, "new_status", userStatus)

	user, err := s.db.IsActive(ctx, userId, userStatus)
	if err != nil {
		return User{}, err
	}

	return user, nil
}

func (s *Service) CreatePR(ctx context.Context, pullRequest PullRequest) (PullRequest, error) {
	s.log.Info("creating pull request", "pr_id", pullRequest.PullRequestID, "author_id", pullRequest.AuthorID)

	user, err := s.db.GetUser(ctx, pullRequest.AuthorID)
	if err != nil {
		return PullRequest{}, err
	}
	team, err := s.db.GetTeam(ctx, user.TeamName)
	if err != nil {
		return PullRequest{}, err
	}
	var reviewers []string

	for _, teamMember := range team.Members {
		if teamMember.IsActive && teamMember.UserID != pullRequest.AuthorID {
			reviewers = append(reviewers, teamMember.UserID)
		}
		if len(reviewers) == 2 {
			break
		}
	}
	if len(reviewers) == 0 {
		return PullRequest{}, ErrNotEnoughReviewers
	}

	pullRequest.AssignedReviewers = reviewers

	err = s.db.AddPR(ctx, pullRequest)
	if err != nil {
		return PullRequest{}, err
	}

	s.log.Info("successfully created pull request",
		"pr_id", pullRequest.PullRequestID,
		"reviewers_count", len(reviewers))

	return pullRequest, nil
}

func (s *Service) Merged(ctx context.Context, prId string) (PullRequest, error) {
	s.log.Info("merged pr", "prId", prId)

	pullRequest, err := s.db.GetPRDetailsWithReviewers(ctx, prId)
	if err != nil {
		return PullRequest{}, err
	}

	if pullRequest.Status == "MERGED" {
		return PullRequest{}, ErrPRAlreadyMerged
	}

	pullRequest, err = s.db.Merged(ctx, prId)
	if err != nil {
		return PullRequest{}, err
	}

	return pullRequest, nil
}

func (s *Service) Reassign(ctx context.Context, reassignReviewer ReassignReviewer) (PullRequest, string, error) {
	s.log.Info("reassigning pull request reviewer",
		"reviewer_id", reassignReviewer.UserID,
		"pull_request_id", reassignReviewer.PRId)

	pullRequest, err := s.db.GetPRDetailsWithReviewers(ctx, reassignReviewer.PRId)
	if err != nil {
		return PullRequest{}, "", err
	}

	if pullRequest.Status == "MERGED" {
		return PullRequest{}, "", ErrPRAlreadyMerged
	}

	user, err := s.db.GetUser(ctx, reassignReviewer.UserID)
	if err != nil {
		return PullRequest{}, "", err
	}

	isReviewer := false
	for _, reviewer := range pullRequest.AssignedReviewers {
		if user.UserID == reviewer {
			isReviewer = true
			break
		}
	}

	if !isReviewer {
		return PullRequest{}, "", ErrReviewerNotAssigned
	}

	team, err := s.db.GetTeam(ctx, user.TeamName)
	if err != nil {
		return PullRequest{}, "", err
	}

	var availableReviewer string
	found := false

	for _, teamMember := range team.Members {
		if teamMember.IsActive && teamMember.UserID != user.UserID && teamMember.UserID != pullRequest.AuthorID {
			isAlreadyReviewer := false
			for _, reviewer := range pullRequest.AssignedReviewers {
				if reviewer == teamMember.UserID {
					isAlreadyReviewer = true
					break
				}
			}

			if !isAlreadyReviewer {
				found = true
				availableReviewer = teamMember.UserID
				break
			}
		}
	}
	if !found {
		return PullRequest{}, "", ErrNoReplacementCandidate
	}

	err = s.db.Reassign(ctx, reassignReviewer, availableReviewer)
	if err != nil {
		return PullRequest{}, "", err
	}

	updatedPR, err := s.db.GetPRDetailsWithReviewers(ctx, reassignReviewer.PRId)
	if err != nil {
		return PullRequest{}, "", err
	}

	s.log.Info("successfully reassigned reviewer",
		"old_reviewer", reassignReviewer.UserID,
		"new_reviewer", availableReviewer,
		"pr_id", reassignReviewer.PRId)

	return updatedPR, availableReviewer, err

}

func (s *Service) GetReview(ctx context.Context, userId string) (UserPullRequest, error) {
	s.log.Info("finding user's assigned pull requests", "user_id", userId)

	_, err := s.db.GetUser(ctx, userId)
	if err != nil {
		return UserPullRequest{}, err
	}

	userPullRequest, err := s.db.GetReview(ctx, userId)
	if err != nil {
		return UserPullRequest{}, err
	}

	return userPullRequest, nil

}

func (s *Service) GetStats(ctx context.Context) (Stats, error) {
	userStats, err := s.db.GetUserReviewStats(ctx)
	if err != nil {
		return nil, err
	}

	prStats, err := s.db.GetPRReviewerCountStats(ctx)
	if err != nil {
		return nil, err
	}

	totalAssignments := 0
	for _, count := range userStats {
		totalAssignments += count
	}

	return Stats{
		"user_assignments":              userStats,
		"pr_reviewer_counts":            prStats,
		"total_assignments":             totalAssignments,
		"unique_users_with_assignments": len(userStats),
		"unique_prs_with_reviewers":     len(prStats),
	}, nil
}
