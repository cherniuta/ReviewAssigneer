package core

import (
	"context"
	"database/sql"
)

type Assigner interface {
	CreateTeam(context.Context, Team) (Team, error)
	GetTeam(context.Context, string) (Team, error)
	IsActive(context.Context, string, bool) (User, error)
	CreatePR(context.Context, PullRequest) (PullRequest, error)
	Merged(context.Context, string) (PullRequest, error)
	Reassign(context.Context, ReassignReviewer) (PullRequest, string, error)
	GetReview(context.Context, string) (UserPullRequest, error)
}

type DB interface {
	AddUserTX(context.Context, *sql.Tx, User) error
	AddTeam(context.Context, Team) error
	AddPR(context.Context, PullRequest) error
	GetTeam(context.Context, string) (Team, error)
	GetUser(context.Context, string) (User, error)
	IsActive(context.Context, string, bool) (User, error)
	Merged(context.Context, string) (PullRequest, error)
	Reassign(context.Context, ReassignReviewer, string) error
	GetPRDetailsWithReviewers(context.Context, string) (PullRequest, error)
	GetReview(context.Context, string) (UserPullRequest, error)
	GetUserReviewStats(context.Context) (map[string]int, error)
	GetPRReviewerCountStats(context.Context) (map[string]int, error)
}
