package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/auth"
)

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		s.log.Error("readiness check failed", "error", err)
		writeError(w, r, http.StatusServiceUnavailable, "database_unavailable", "Database is not reachable.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

type devLoginRequest struct {
	Email string `json:"email"`
}

// devLogin exists only when APP_ENV=development.
func (s *Server) devLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var req devLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_json", "Send a JSON body like {\"email\":\"you@example.com\"}.")
		return
	}
	email := strings.TrimSpace(req.Email)
	if len(email) < 3 || len(email) > 254 || !strings.Contains(email, "@") {
		writeError(w, r, http.StatusBadRequest, "invalid_email", "Enter a valid email address.")
		return
	}

	user, err := s.sessions.UpsertDevUser(r.Context(), email)
	if err != nil {
		s.log.Error("dev login failed", "error", err, "request_id", requestIDFrom(r.Context()))
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Something went wrong.")
		return
	}
	token, expires, err := s.sessions.Create(r.Context(), user.ID)
	if err != nil {
		s.log.Error("create session failed", "error", err, "request_id", requestIDFrom(r.Context()))
		writeError(w, r, http.StatusInternalServerError, "internal_error", "Something went wrong.")
		return
	}

	http.SetCookie(w, s.sessionCookie(token, expires))
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(s.cfg.SessionCookieName); err == nil {
		if err := s.sessions.Delete(r.Context(), c.Value); err != nil {
			s.log.Error("delete session failed", "error", err)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !s.cfg.IsDev(),
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) sessionCookie(value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     s.cfg.SessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,                 // JavaScript cannot read it
		Secure:   !s.cfg.IsDev(),       // HTTPS only outside development
		SameSite: http.SameSiteLaxMode, // not sent on cross-site POSTs
	}
}
