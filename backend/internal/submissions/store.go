package submissions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrMilestoneNotFound = errors.New("milestone not found in this project")
)

type Submission struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	MilestoneID   *string   `json:"milestone_id"`
	Version       int       `json:"version"`
	Files         []File    `json:"files"`
	TotalBytes    int       `json:"total_bytes"`
	ContentSHA256 string    `json:"content_sha256"`
	LearnerNote   string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
}

// Summary is the list view: no file contents.
type Summary struct {
	ID          string    `json:"id"`
	Version     int       `json:"version"`
	MilestoneID *string   `json:"milestone_id"`
	FileCount   int       `json:"file_count"`
	TotalBytes  int       `json:"total_bytes"`
	LearnerNote string    `json:"note"`
	CreatedAt   time.Time `json:"created_at"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// querier is satisfied by both the pool and a transaction.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Create stores a new immutable version. The project row is locked while the
// version number is chosen, so concurrent uploads cannot collide.
// If the content is identical to the latest version, nothing new is stored and
// created is false: a retried request never makes a duplicate.
func (s *Store) Create(ctx context.Context, userID, projectID string, milestoneID *string, p Prepared) (sub Submission, created bool, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Submission{}, false, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var lockedID string
	err = tx.QueryRow(ctx, `
		SELECT id::text FROM projects
		WHERE id = $1::uuid AND user_id = $2::uuid AND deleted_at IS NULL
		FOR UPDATE`, projectID, userID).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Submission{}, false, ErrNotFound
	}
	if err != nil {
		return Submission{}, false, fmt.Errorf("lock project: %w", err)
	}

	if milestoneID != nil {
		var mid string
		err = tx.QueryRow(ctx,
			`SELECT id::text FROM milestones WHERE id = $1::uuid AND project_id = $2::uuid`,
			*milestoneID, projectID).Scan(&mid)
		if errors.Is(err, pgx.ErrNoRows) {
			return Submission{}, false, ErrMilestoneNotFound
		}
		if err != nil {
			return Submission{}, false, fmt.Errorf("check milestone: %w", err)
		}
	}

	latest := 0
	var latestHash string
	var latestMilestone *string
	err = tx.QueryRow(ctx, `
		SELECT version, content_sha256, milestone_id::text
		FROM submissions WHERE project_id = $1::uuid
		ORDER BY version DESC LIMIT 1`, projectID).Scan(&latest, &latestHash, &latestMilestone)
	switch {
	case err == nil:
		if latestHash == p.SHA256 && sameID(latestMilestone, milestoneID) {
			existing, rerr := readSubmission(ctx, tx, userID, projectID, latest)
			return existing, false, rerr
		}
	case errors.Is(err, pgx.ErrNoRows):
		latest = 0
	default:
		return Submission{}, false, fmt.Errorf("read latest version: %w", err)
	}

	filesJSON, err := json.Marshal(p.Files)
	if err != nil {
		return Submission{}, false, fmt.Errorf("encode files: %w", err)
	}

	next := latest + 1
	_, err = tx.Exec(ctx, `
		INSERT INTO submissions (project_id, milestone_id, version, files, total_bytes, content_sha256, learner_note)
		VALUES ($1::uuid, $2::uuid, $3, $4::jsonb, $5, $6, $7)`,
		projectID, milestoneID, next, string(filesJSON), p.TotalBytes, p.SHA256, p.Note)
	if err != nil {
		return Submission{}, false, fmt.Errorf("insert submission: %w", err)
	}

	sub, err = readSubmission(ctx, tx, userID, projectID, next)
	if err != nil {
		return Submission{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Submission{}, false, fmt.Errorf("commit: %w", err)
	}
	return sub, true, nil
}

// List returns version summaries, newest first, for a project the caller owns.
func (s *Store) List(ctx context.Context, userID, projectID string) ([]Summary, error) {
	var owned string
	err := s.pool.QueryRow(ctx, `
		SELECT id::text FROM projects
		WHERE id = $1::uuid AND user_id = $2::uuid AND deleted_at IS NULL`,
		projectID, userID).Scan(&owned)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("check project: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id::text, version, milestone_id::text, jsonb_array_length(files),
		       total_bytes, learner_note, created_at
		FROM submissions
		WHERE project_id = $1::uuid
		ORDER BY version DESC
		LIMIT 200`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list submissions: %w", err)
	}
	defer rows.Close()

	out := []Summary{}
	for rows.Next() {
		var m Summary
		if err := rows.Scan(&m.ID, &m.Version, &m.MilestoneID, &m.FileCount, &m.TotalBytes, &m.LearnerNote, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan submission: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list submissions: %w", err)
	}
	return out, nil
}

// Get returns one version with its files.
func (s *Store) Get(ctx context.Context, userID, projectID string, version int) (Submission, error) {
	return readSubmission(ctx, s.pool, userID, projectID, version)
}

// readSubmission joins projects so ownership is enforced in the same query.
func readSubmission(ctx context.Context, q querier, userID, projectID string, version int) (Submission, error) {
	const query = `
		SELECT s.id::text, s.project_id::text, s.milestone_id::text, s.version, s.files,
		       s.total_bytes, s.content_sha256, s.learner_note, s.created_at
		FROM submissions s
		JOIN projects p ON p.id = s.project_id
		WHERE s.project_id = $1::uuid AND p.user_id = $2::uuid
		  AND p.deleted_at IS NULL AND s.version = $3`

	var sub Submission
	err := q.QueryRow(ctx, query, projectID, userID, version).Scan(
		&sub.ID, &sub.ProjectID, &sub.MilestoneID, &sub.Version, &sub.Files,
		&sub.TotalBytes, &sub.ContentSHA256, &sub.LearnerNote, &sub.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Submission{}, ErrNotFound
	}
	if err != nil {
		return Submission{}, fmt.Errorf("read submission: %w", err)
	}
	return sub, nil
}

func sameID(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return strings.EqualFold(*a, *b)
}
