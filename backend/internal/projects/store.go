package projects

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrTooManyMilestones = errors.New("milestone limit reached")
)

const MaxMilestones = 20

type Project struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Brief     string    `json:"brief"`
	Language  string    `json:"language"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Milestone struct {
	ID               string `json:"id"`
	Position         int    `json:"position"`
	Title            string `json:"title"`
	Goal             string `json:"goal"`
	EvidenceRequired string `json:"evidence_required"`
	Status           string `json:"status"`
	Version          int    `json:"version"`
}

type Detail struct {
	Project
	Milestones []Milestone `json:"milestones"`
}

type NewProject struct {
	Title    string
	Brief    string
	Language string
}

type NewMilestone struct {
	Title            string
	Goal             string
	EvidenceRequired string
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Create makes a project owned by userID.
func (s *Store) Create(ctx context.Context, userID string, in NewProject) (Project, error) {
	const q = `
		INSERT INTO projects (user_id, title, brief, language)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id::text, title, brief, language, status, created_at, updated_at`

	var p Project
	err := s.pool.QueryRow(ctx, q, userID, in.Title, in.Brief, in.Language).
		Scan(&p.ID, &p.Title, &p.Brief, &p.Language, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return Project{}, fmt.Errorf("create project: %w", err)
	}
	return p, nil
}

// List returns only the caller's own projects, newest first.
func (s *Store) List(ctx context.Context, userID string) ([]Project, error) {
	const q = `
		SELECT id::text, title, brief, language, status, created_at, updated_at
		FROM projects
		WHERE user_id = $1::uuid AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 100`

	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	out := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Title, &p.Brief, &p.Language, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return out, nil
}

// Get returns one project with its milestones. A project that belongs to
// someone else is reported as ErrNotFound, exactly like one that does not exist.
func (s *Store) Get(ctx context.Context, userID, projectID string) (Detail, error) {
	const q = `
		SELECT id::text, title, brief, language, status, created_at, updated_at
		FROM projects
		WHERE id = $1::uuid AND user_id = $2::uuid AND deleted_at IS NULL`

	var d Detail
	err := s.pool.QueryRow(ctx, q, projectID, userID).
		Scan(&d.ID, &d.Title, &d.Brief, &d.Language, &d.Status, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, ErrNotFound
	}
	if err != nil {
		return Detail{}, fmt.Errorf("get project: %w", err)
	}

	const mq = `
		SELECT id::text, position, title, goal, evidence_required, status, version
		FROM milestones
		WHERE project_id = $1::uuid
		ORDER BY position`

	rows, err := s.pool.Query(ctx, mq, projectID)
	if err != nil {
		return Detail{}, fmt.Errorf("get milestones: %w", err)
	}
	defer rows.Close()

	d.Milestones = []Milestone{}
	for rows.Next() {
		var m Milestone
		if err := rows.Scan(&m.ID, &m.Position, &m.Title, &m.Goal, &m.EvidenceRequired, &m.Status, &m.Version); err != nil {
			return Detail{}, fmt.Errorf("scan milestone: %w", err)
		}
		d.Milestones = append(d.Milestones, m)
	}
	if err := rows.Err(); err != nil {
		return Detail{}, fmt.Errorf("get milestones: %w", err)
	}
	return d, nil
}

// AddMilestone appends a milestone to the caller's project. The project row is
// locked for the duration so two simultaneous requests cannot get the same position.
func (s *Store) AddMilestone(ctx context.Context, userID, projectID string, in NewMilestone) (Milestone, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Milestone{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var lockedID string
	err = tx.QueryRow(ctx, `
		SELECT id::text FROM projects
		WHERE id = $1::uuid AND user_id = $2::uuid AND deleted_at IS NULL
		FOR UPDATE`, projectID, userID).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Milestone{}, ErrNotFound
	}
	if err != nil {
		return Milestone{}, fmt.Errorf("lock project: %w", err)
	}

	var count int64
	var next int
	err = tx.QueryRow(ctx, `
		SELECT count(*), coalesce(max(position), 0) + 1
		FROM milestones WHERE project_id = $1::uuid`, projectID).Scan(&count, &next)
	if err != nil {
		return Milestone{}, fmt.Errorf("count milestones: %w", err)
	}
	if count >= MaxMilestones {
		return Milestone{}, ErrTooManyMilestones
	}

	var m Milestone
	err = tx.QueryRow(ctx, `
		INSERT INTO milestones (project_id, position, title, goal, evidence_required)
		VALUES ($1::uuid, $2, $3, $4, $5)
		RETURNING id::text, position, title, goal, evidence_required, status, version`,
		projectID, next, in.Title, in.Goal, in.EvidenceRequired).
		Scan(&m.ID, &m.Position, &m.Title, &m.Goal, &m.EvidenceRequired, &m.Status, &m.Version)
	if err != nil {
		return Milestone{}, fmt.Errorf("insert milestone: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Milestone{}, fmt.Errorf("commit: %w", err)
	}
	return m, nil
}
