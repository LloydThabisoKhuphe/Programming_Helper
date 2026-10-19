package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/auth"
	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/config"
)

type Server struct {
	cfg      config.Config
	pool     *pgxpool.Pool
	sessions *auth.Store
	log      *slog.Logger
}

func New(cfg config.Config, pool *pgxpool.Pool, sessions *auth.Store, log *slog.Logger) http.Handler {
	s := &Server{cfg: cfg, pool: pool, sessions: sessions, log: log}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	if cfg.IsDev() {
		mux.HandleFunc("POST /v1/dev/login", s.devLogin)
	}
	mux.HandleFunc("POST /v1/logout", s.logout)
	mux.Handle("GET /v1/me", s.requireAuth(http.HandlerFunc(s.me)))

	// Outermost runs first: request ID -> logging -> panic recovery -> origin check -> routes.
	var h http.Handler = mux
	h = s.originCheck(h)
	h = s.recoverer(h)
	h = s.logRequests(h)
	h = s.requestID(h)
	return h
}
