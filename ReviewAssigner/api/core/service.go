package core

import (
	"context"
	"fmt"
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
		if teamMember.IsActive {
			reviewers = append(reviewers, teamMember.UserID)
		}
		if len(reviewers) == 2 {
			break
		}
	}
	if len(reviewers) == 0 {
		return PullRequest{}, fmt.Errorf("no active reviewers found in team '%s'", user.TeamName)
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
		return PullRequest{}, fmt.Errorf("pull request %s is already merged", prId)
	}

	pullRequest, err = s.db.Merged(ctx, prId)
	if err != nil {
		return PullRequest{}, err
	}

	return pullRequest, nil
}

func (s *Service) Reassign(ctx context.Context, reassignReviewer ReassignReviewer) (ReassignReviewer, error) {
	s.log.Info("reassigning pull request reviewer",
		"reviewer_id", reassignReviewer.UserID,
		"pull_request_id", reassignReviewer.PR.PullRequestID)

	pullRequest, err := s.db.GetPRDetailsWithReviewers(ctx, reassignReviewer.PR.PullRequestID)
	if err != nil {
		return ReassignReviewer{}, err
	}

	user, err := s.db.GetUser(ctx, reassignReviewer.UserID)
	if err != nil {
		return ReassignReviewer{}, err
	}

	isReviewer := false
	for _, reviewer := range pullRequest.AssignedReviewers {
		if user.UserID == reviewer {
			isReviewer = true
			break
		}
	}

	if !isReviewer {
		return ReassignReviewer{}, fmt.Errorf("user %s is not assigned as reviewer for pull request %s",
			reassignReviewer.UserID, reassignReviewer.PR.PullRequestID)
	}

	team, err := s.db.GetTeam(ctx, user.TeamName)
	if err != nil {
		return ReassignReviewer{}, err
	}

	var availableReviewer string
	found := false

	for _, teamMember := range team.Members {
		if teamMember.IsActive && teamMember.UserID != user.UserID {
			// Проверяем, что этот член команды еще не ревьювер
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
				break // 5. Берем первого подходящего
			}
		}
	}
	if !found {
		return ReassignReviewer{}, fmt.Errorf("no available active team member {pr_id}", reassignReviewer.PR.PullRequestID)
	}

	err = s.db.Reassign(ctx, reassignReviewer, availableReviewer)
	if err != nil {
		return ReassignReviewer{}, err
	}
	updatedReviewer := reassignReviewer
	updatedReviewer.UserID = availableReviewer

	s.log.Info("successfully reassigned reviewer",
		"old_reviewer", reassignReviewer.UserID,
		"new_reviewer", availableReviewer,
		"pr_id", reassignReviewer.PR.PullRequestID)

	return updatedReviewer, err

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
