package db

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"log/slog"
	"review-assigner/core"
	"strings"
)

type DB struct {
	log  *slog.Logger
	conn *sqlx.DB
}

func New(log *slog.Logger, address string) (*DB, error) {

	db, err := sqlx.Connect("pgx", address)
	if err != nil {
		log.Error("connection problem", "address", address, "error", err)
		return nil, err
	}

	return &DB{
		log:  log,
		conn: db,
	}, nil
}

func isUniqueConstraintError(err error) bool {
	if pgErr, ok := err.(*pq.Error); ok {
		return pgErr.Code == "23505"
	}
	return false
}

func (db *DB) AddUserTX(ctx context.Context, tx *sql.Tx, user core.User) error {

	stmt, err := tx.Prepare("INSERT INTO users (id,name,team_name,active) VALUES ($1, $2,$3,$4)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, user.UserID, user.Username, user.TeamName, user.IsActive)

	return err
}

func (db *DB) AddTeam(ctx context.Context, team core.Team) error {

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	stmt, err := tx.Prepare("INSERT INTO teams (name) VALUES ($1)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, team.TeamName)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			return core.ErrTeamAlreadyExists
		}
		return err
	}

	for _, member := range team.Members {

		user := core.User{member.UserID, member.Username, team.TeamName, member.IsActive}
		err = db.AddUserTX(ctx, tx, user)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *DB) AddPR(ctx context.Context, pullRequest core.PullRequest) error {

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	prstmt, err := tx.Prepare("INSERT INTO pull_request (id,title,author_id,state) VALUES ($1, $2,$3,$4)")
	if err != nil {
		return err
	}
	defer prstmt.Close()

	_, err = prstmt.ExecContext(ctx, pullRequest.PullRequestID, pullRequest.PullRequestName, pullRequest.AuthorID, "OPEN")
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			return core.ErrPRAAlreadyExists
		}
	}

	reviewerStmt, err := tx.Prepare("INSERT INTO pr_reviewers (pr_id, reviewer_id) VALUES ($1, $2)")
	if err != nil {
		return fmt.Errorf("prepare reviewer statement: %w", err)
	}
	defer reviewerStmt.Close()

	for _, reviewer := range pullRequest.AssignedReviewers {
		_, err = reviewerStmt.ExecContext(ctx, pullRequest.PullRequestID, reviewer)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *DB) GetTeam(ctx context.Context, teamName string) (core.Team, error) {

	var (
		exists bool
		team   core.Team
	)

	err := db.conn.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM teams WHERE name = $1)", teamName).Scan(&exists)
	if err != nil {
		return core.Team{}, err
	}
	if !exists {
		return core.Team{}, core.ErrTeamNotFound
	}
	team.TeamName = teamName

	rows, err := db.conn.QueryContext(ctx,
		"SELECT id,name,active FROM users WHERE team_name = $1", teamName)
	if err != nil {
		return core.Team{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var teamMembers core.TeamMember
		err = rows.Scan(&teamMembers.UserID, &teamMembers.Username, &teamMembers.IsActive)
		if err != nil {
			return core.Team{}, err
		}
		team.Members = append(team.Members, teamMembers)
	}

	if err = rows.Err(); err != nil {
		return core.Team{}, err
	}

	return team, nil

}

func (db *DB) GetUser(ctx context.Context, userId string) (core.User, error) {
	var user core.User

	err := db.conn.QueryRowContext(ctx,
		"SELECT id, name, team_name, active FROM users WHERE id = $1",
		userId,
	).Scan(&user.UserID, &user.Username, &user.TeamName, &user.IsActive)

	if err != nil {
		if err == sql.ErrNoRows {
			return core.User{}, core.ErrUserNotFound
		}
		return core.User{}, err
	}

	return user, nil
}

func (db *DB) IsActive(ctx context.Context, userId string, status bool) (core.User, error) {
	var user core.User

	err := db.conn.QueryRowContext(
		ctx,
		`UPDATE users SET active = $1
         WHERE id = $2 
         RETURNING id, name, team_name, active`,
		status, userId,
	).Scan(&user.UserID, &user.Username, &user.TeamName, &user.IsActive)

	if err != nil {
		if err == sql.ErrNoRows {
			return core.User{}, core.ErrUserNotFound
		}
		return core.User{}, fmt.Errorf("failed to update user: %w", err)
	}

	return user, nil
}

func (db *DB) Merged(ctx context.Context, prId string) (core.PullRequest, error) {
	var pullRequest core.PullRequest

	err := db.conn.QueryRowContext(
		ctx,
		`UPDATE pull_request SET state = 'MERGED'
         WHERE id = $1 and state='OPEN'
         RETURNING id, title, author_id, state`,
		prId,
	).Scan(&pullRequest.PullRequestID, &pullRequest.PullRequestName, &pullRequest.AuthorID, &pullRequest.Status)

	if err != nil {
		if err == sql.ErrNoRows {
			return core.PullRequest{}, fmt.Errorf("pr %s not found", prId)
		}
		return core.PullRequest{}, fmt.Errorf("failed to update pr: %w", err)
	}

	pullRequest.AssignedReviewers, err = db.getReviewers(ctx, prId)
	if err != nil {
		return core.PullRequest{}, err
	}

	return pullRequest, nil
}

func (db *DB) GetPRDetailsWithReviewers(ctx context.Context, prId string) (core.PullRequest, error) {
	var pullRequest core.PullRequest

	err := db.conn.QueryRowContext(
		ctx,
		`SELECT id, title, author_id, state 
         FROM pull_request
         WHERE id = $1`,
		prId,
	).Scan(&pullRequest.PullRequestID, &pullRequest.PullRequestName, &pullRequest.AuthorID, &pullRequest.Status)

	if err != nil {
		if err == sql.ErrNoRows {
			return core.PullRequest{}, core.ErrPRNotFound
		}
		return core.PullRequest{}, fmt.Errorf("failed to get pr: %w", err)
	}

	reviewers, err := db.getReviewers(ctx, prId)
	if err != nil {
		return core.PullRequest{}, fmt.Errorf("failed to get reviewers: %w", err)
	}
	pullRequest.AssignedReviewers = reviewers

	return pullRequest, nil
}

func (db *DB) getReviewers(ctx context.Context, prId string) ([]string, error) {
	var reviewers []string

	rows, err := db.conn.QueryContext(
		ctx,
		`SELECT reviewer_id FROM pr_reviewers WHERE pr_id = $1`,
		prId,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get reviewers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var reviewerID string
		if err = rows.Scan(&reviewerID); err != nil {
			return nil, fmt.Errorf("failed to scan reviewer: %w", err)
		}
		reviewers = append(reviewers, reviewerID)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating reviewers: %w", err)
	}

	return reviewers, nil
}

func (db *DB) Reassign(ctx context.Context, oldReviewer core.ReassignReviewer, newReviewer string) error {

	result, err := db.conn.ExecContext(
		ctx,
		`UPDATE pr_reviewers SET reviewer_id = $1
         WHERE pr_id = $2 AND reviewer_id = $3`,
		newReviewer, oldReviewer.PRId, oldReviewer.UserID,
	)

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("pr %s not found or reviewer not assigned", oldReviewer.PRId)
	}

	return nil
}

func (db *DB) GetReview(ctx context.Context, userId string) (core.UserPullRequest, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT pr.id, pr.title, pr.author_id, pr.state, pr_reviewers.reviewer_id 
         FROM pull_request pr 
         JOIN pr_reviewers ON pr.id = pr_reviewers.pr_id 
         WHERE pr_reviewers.reviewer_id = $1`,
		userId)
	if err != nil {
		return core.UserPullRequest{}, fmt.Errorf("failed to query reviews: %w", err)
	}
	defer rows.Close()

	var userPullRequest core.UserPullRequest
	userPullRequest.UserID = userId

	prMap := make(map[string]*core.PullRequest)

	for rows.Next() {
		var prID, title, authorID, state, reviewerID string

		err = rows.Scan(&prID, &title, &authorID, &state, &reviewerID)
		if err != nil {
			return core.UserPullRequest{}, fmt.Errorf("failed to scan row: %w", err)
		}

		if _, exists := prMap[prID]; !exists {
			prMap[prID] = &core.PullRequest{
				PullRequestID:   prID,
				PullRequestName: title,
				AuthorID:        authorID,
				Status:          state,
			}
		}

	}

	if err = rows.Err(); err != nil {
		return core.UserPullRequest{}, fmt.Errorf("error iterating rows: %w", err)
	}

	for _, pr := range prMap {
		userPullRequest.PullRequest = append(userPullRequest.PullRequest, *pr)
	}

	return userPullRequest, nil
}

func (db *DB) GetUserReviewStats(ctx context.Context) (map[string]int, error) {
	stats := make(map[string]int)

	rows, err := db.conn.QueryContext(ctx, `
        SELECT reviewer_id, COUNT(*) as assignment_count 
        FROM pr_reviewers 
        GROUP BY reviewer_id
        ORDER BY assignment_count DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var userID string
		var count int
		if err := rows.Scan(&userID, &count); err != nil {
			return nil, err
		}
		stats[userID] = count
	}

	return stats, nil
}

func (db *DB) GetPRReviewerCountStats(ctx context.Context) (map[string]int, error) {
	stats := make(map[string]int)

	rows, err := db.conn.QueryContext(ctx, `
        SELECT pr_id, COUNT(*) as reviewer_count 
        FROM pr_reviewers 
        GROUP BY pr_id
        ORDER BY reviewer_count DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var prID string
		var count int
		if err := rows.Scan(&prID, &count); err != nil {
			return nil, err
		}
		stats[prID] = count
	}

	return stats, nil
}
