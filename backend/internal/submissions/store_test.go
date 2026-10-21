package submissions_test

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
	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/submissions"
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

func prep(t *testing.T, content string) submissions.Prepared {
	t.Helper()
	p, fields := submissions.Prepare([]submissions.FileInput{{Path: "expenses.js", Content: content}}, "")
	if len(fields) != 0 {
		t.Fatalf("prepare: %v", fields)
	}
	return p
}

func TestSubmissionVersioningAndIsolation(t *testing.T) {
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
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1::uuid, $2::uuid)`, userA, userB)
	})

	projStore := projects.NewStore(pool)
	store := submissions.NewStore(pool)

	proj, err := projStore.Create(ctx, userA, projects.NewProject{Title: "Expense tracker", Brief: "x", Language: "javascript"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	ms, err := projStore.AddMilestone(ctx, userA, proj.ID, projects.NewMilestone{Title: "Validate an amount"})
	if err != nil {
		t.Fatalf("create milestone: %v", err)
	}

	// B cannot upload to A's project.
	if _, _, err := store.Create(ctx, userB, proj.ID, nil, prep(t, "x")); !errors.Is(err, submissions.ErrNotFound) {
		t.Fatalf("B create on A's project: want ErrNotFound, got %v", err)
	}

	// A's first upload is version 1.
	v1, created, err := store.Create(ctx, userA, proj.ID, &ms.ID, prep(t, "const a = 1;"))
	if err != nil || !created || v1.Version != 1 {
		t.Fatalf("v1: err=%v created=%v version=%d", err, created, v1.Version)
	}

	// The same content again is not a new version.
	again, created, err := store.Create(ctx, userA, proj.ID, &ms.ID, prep(t, "const a = 1;"))
	if err != nil || created || again.Version != 1 || again.ID != v1.ID {
		t.Fatalf("retry: err=%v created=%v version=%d", err, created, again.Version)
	}

	// Changed content becomes version 2.
	v2, created, err := store.Create(ctx, userA, proj.ID, &ms.ID, prep(t, "const a = 2;"))
	if err != nil || !created || v2.Version != 2 {
		t.Fatalf("v2: err=%v created=%v version=%d", err, created, v2.Version)
	}

	// A milestone from nowhere is refused.
	bogus := "99999999-9999-9999-9999-999999999999"
	if _, _, err := store.Create(ctx, userA, proj.ID, &bogus, prep(t, "const a = 3;")); !errors.Is(err, submissions.ErrMilestoneNotFound) {
		t.Fatalf("bogus milestone: want ErrMilestoneNotFound, got %v", err)
	}

	// A can list and read; B cannot.
	list, err := store.List(ctx, userA, proj.ID)
	if err != nil || len(list) != 2 || list[0].Version != 2 {
		t.Fatalf("A list: err=%v len=%d", err, len(list))
	}
	got, err := store.Get(ctx, userA, proj.ID, 1)
	if err != nil || len(got.Files) != 1 || got.Files[0].Content != "const a = 1;" {
		t.Fatalf("A get v1: err=%v", err)
	}
	if _, err := store.List(ctx, userB, proj.ID); !errors.Is(err, submissions.ErrNotFound) {
		t.Fatalf("B list: want ErrNotFound, got %v", err)
	}
	if _, err := store.Get(ctx, userB, proj.ID, 1); !errors.Is(err, submissions.ErrNotFound) {
		t.Fatalf("B get: want ErrNotFound, got %v", err)
	}

	// The database itself refuses edits.
	if _, err := pool.Exec(ctx, `UPDATE submissions SET learner_note = 'edited' WHERE id = $1::uuid`, v1.ID); err == nil {
		t.Fatal("UPDATE on a submission should have been rejected")
	}

	// Deleting the project must still work, even though submissions point at a milestone.
	if _, err := pool.Exec(ctx, `DELETE FROM projects WHERE id = $1::uuid`, proj.ID); err != nil {
		t.Fatalf("delete project with submissions: %v", err)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM submissions WHERE project_id = $1::uuid`, proj.ID).Scan(&left); err != nil || left != 0 {
		t.Fatalf("submissions should be gone after project delete: err=%v left=%d", err, left)
	}
}
