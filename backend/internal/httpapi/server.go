package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/auth"
	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/config"
	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/projects"
	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/submissions"
)

type Server struct {
	cfg         config.Config
	pool        *pgxpool.Pool
	sessions    *auth.Store
	projects    *projects.Store
	submissions *submissions.Store
	log         *slog.Logger
}

func New(
	cfg config.Config,
	pool *pgxpool.Pool,
	sessions *auth.Store,
	projectStore *projects.Store,
	submissionStore *submissions.Store,
	log *slog.Logger,
) http.Handler {
	s := &Server{
		cfg: cfg, pool: pool, sessions: sessions,
		projects: projectStore, submissions: submissionStore, log: log,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	if cfg.IsDev() {
		mux.HandleFunc("POST /v1/dev/login", s.devLogin)
	}
	mux.HandleFunc("POST /v1/logout", s.logout)

	authed := func(h http.HandlerFunc) http.Handler { return s.requireAuth(h) }
	mux.Handle("GET /v1/me", authed(s.me))

	mux.Handle("POST /v1/projects", authed(s.createProject))
	mux.Handle("GET /v1/projects", authed(s.listProjects))
	mux.Handle("GET /v1/projects/{id}", authed(s.getProject))
	mux.Handle("POST /v1/projects/{id}/milestones", authed(s.createMilestone))

	mux.Handle("POST /v1/projects/{id}/submissions", authed(s.createSubmission))
	mux.Handle("GET /v1/projects/{id}/submissions", authed(s.listSubmissions))
	mux.Handle("GET /v1/projects/{id}/submissions/{version}", authed(s.getSubmission))

	// Outermost runs first: request ID -> logging -> panic recovery -> origin check -> routes.
	var h http.Handler = mux
	h = s.originCheck(h)
	h = s.recoverer(h)
	h = s.logRequests(h)
	h = s.requestID(h)
	return h
}
