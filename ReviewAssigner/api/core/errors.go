package core

import "errors"

var (
	ErrTeamAlreadyExists      = errors.New("team_name already exists")
	ErrTeamNotFound           = errors.New("team not found")
	ErrUserNotFound           = errors.New("user not found")
	ErrPRAAlreadyExists       = errors.New("PR already exists")
	ErrNotEnoughReviewers     = errors.New("not enough active reviewers in team")
	ErrPRNotFound             = errors.New("PR not found")
	ErrPRAlreadyMerged        = errors.New("cannot reassign on merged PR")
	ErrReviewerNotAssigned    = errors.New("reviewer is not assigned to this PR")
	ErrNoReplacementCandidate = errors.New("no active replacement candidate in team")
)
