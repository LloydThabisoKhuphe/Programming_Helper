package projects_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/db"
	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/projects"
)

func newUser(t *testing.T, pool *pgxpool.Pool, label string) string {
	t.Helper()
	var id string
	subject := fmt.Sprintf("test:%s:%d", label, time.Now().UnixNano())
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (oidc_subject) VALUES ($1) RETURNING id::text`, subject).Scan(&id)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return id
}

func TestOwnershipIsolation(t *testing.T) {
	_ = godotenv.Load("../../../.env")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	userA := newUser(t, pool, "a")
	userB := newUser(t, pool, "b")
	t.Cleanup(func() {
		// Deleting the users cascades to their projects and milestones.
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1::uuid, $2::uuid)`, userA, userB)
	})

	store := projects.NewStore(pool)

	p, err := store.Create(ctx, userA, projects.NewProject{Title: "Expense tracker", Brief: "Track spending", Language: "javascript"})
	if err != nil {
		t.Fatalf("A create: %v", err)
	}

	// The owner can read it.
	if _, err := store.Get(ctx, userA, p.ID); err != nil {
		t.Fatalf("A get own project: %v", err)
	}

	// Another user must get "not found", never the data.
	if _, err := store.Get(ctx, userB, p.ID); !errors.Is(err, projects.ErrNotFound) {
		t.Fatalf("B get A's project: want ErrNotFound, got %v", err)
	}

	// B's list must not include A's project.
	listB, err := store.List(ctx, userB)
	if err != nil {
		t.Fatalf("B list: %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("B list: want 0 projects, got %d", len(listB))
	}

	// A's list includes it.
	listA, err := store.List(ctx, userA)
	if err != nil {
		t.Fatalf("A list: %v", err)
	}
	if len(listA) != 1 || listA[0].ID != p.ID {
		t.Fatalf("A list: want exactly the new project, got %d", len(listA))
	}

	// B must not be able to add milestones to A's project.
	_, err = store.AddMilestone(ctx, userB, p.ID, projects.NewMilestone{Title: "Sneaky"})
	if !errors.Is(err, projects.ErrNotFound) {
		t.Fatalf("B add milestone to A's project: want ErrNotFound, got %v", err)
	}

	// A's milestones get positions 1 and 2, in order.
	m1, err := store.AddMilestone(ctx, userA, p.ID, projects.NewMilestone{Title: "Define the fields"})
	if err != nil {
		t.Fatalf("A add milestone 1: %v", err)
	}
	m2, err := store.AddMilestone(ctx, userA, p.ID, projects.NewMilestone{Title: "Validate an amount"})
	if err != nil {
		t.Fatalf("A add milestone 2: %v", err)
	}
	if m1.Position != 1 || m2.Position != 2 {
		t.Fatalf("positions: want 1 and 2, got %d and %d", m1.Position, m2.Position)
	}

	d, err := store.Get(ctx, userA, p.ID)
	if err != nil {
		t.Fatalf("A get with milestones: %v", err)
	}
	if len(d.Milestones) != 2 || d.Milestones[0].Title != "Define the fields" {
		t.Fatalf("milestones not returned in order: %+v", d.Milestones)
	}
}
