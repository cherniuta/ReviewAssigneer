package core

type UserStatus string

const (
	StatusActive UserStatus = "true"
	StatusSleep  UserStatus = "false"
)

type TeamMember struct {
	UserID   string
	Username string
	IsActive bool
}

type Team struct {
	TeamName string
	Members  []TeamMember
}

type User struct {
	UserID   string
	Username string
	TeamName string
	IsActive bool
}

type PullRequest struct {
	PullRequestID     string
	PullRequestName   string
	AuthorID          string
	Status            string
	AssignedReviewers []string
	CreatedAt         *string
	MergedAt          *string
}

type UserPullRequest struct {
	UserID      string
	PullRequest []PullRequest
}

type ReassignReviewer struct {
	PRId   string
	UserID string
}
