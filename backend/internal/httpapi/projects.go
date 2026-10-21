package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/auth"
	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/projects"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var allowedLanguages = map[string]bool{"javascript": true, "typescript": true, "go": true}

// decodeJSON reads a size-limited JSON body and rejects unknown fields.
// It writes the error response itself and returns false on failure.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "Request body is too large.")
		} else {
			writeError(w, r, http.StatusBadRequest, "invalid_json", "Send a valid JSON body with known fields only.")
		}
		return false
	}
	return true
}

func writeValidation(w http.ResponseWriter, r *http.Request, fields map[string]string) {
	writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"error": map[string]any{
			"code":       "validation_failed",
			"message":    "Some fields need attention.",
			"fields":     fields,
			"request_id": requestIDFrom(r.Context()),
		},
	})
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, msg string, err error) {
	s.log.Error(msg, "error", err, "request_id", requestIDFrom(r.Context()))
	writeError(w, r, http.StatusInternalServerError, "internal_error", "Something went wrong.")
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

type createProjectRequest struct {
	Title    string `json:"title"`
	Brief    string `json:"brief"`
	Language string `json:"language"`
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())

	var req createProjectRequest
	if !decodeJSON(w, r, &req, 16*1024) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Brief = strings.TrimSpace(req.Brief)
	req.Language = strings.ToLower(strings.TrimSpace(req.Language))

	fields := map[string]string{}
	if n := runeLen(req.Title); n < 1 || n > 120 {
		fields["title"] = "Use 1 to 120 characters."
	}
	if n := runeLen(req.Brief); n < 1 || n > 4000 {
		fields["brief"] = "Use 1 to 4000 characters."
	}
	if !allowedLanguages[req.Language] {
		fields["language"] = "Choose javascript, typescript or go."
	}
	if len(fields) > 0 {
		writeValidation(w, r, fields)
		return
	}

	p, err := s.projects.Create(r.Context(), u.ID, projects.NewProject{
		Title: req.Title, Brief: req.Brief, Language: req.Language,
	})
	if err != nil {
		s.serverError(w, r, "create project failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	list, err := s.projects.List(r.Context(), u.ID)
	if err != nil {
		s.serverError(w, r, "list projects failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": list})
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, r, http.StatusNotFound, "not_found", "Project not found.")
		return
	}

	d, err := s.projects.Get(r.Context(), u.ID, id)
	if errors.Is(err, projects.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "Project not found.")
		return
	}
	if err != nil {
		s.serverError(w, r, "get project failed", err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

type createMilestoneRequest struct {
	Title            string `json:"title"`
	Goal             string `json:"goal"`
	EvidenceRequired string `json:"evidence_required"`
}

func (s *Server) createMilestone(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	id := r.PathValue("id")
	if !uuidRe.MatchString(id) {
		writeError(w, r, http.StatusNotFound, "not_found", "Project not found.")
		return
	}

	var req createMilestoneRequest
	if !decodeJSON(w, r, &req, 16*1024) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Goal = strings.TrimSpace(req.Goal)
	req.EvidenceRequired = strings.TrimSpace(req.EvidenceRequired)

	fields := map[string]string{}
	if n := runeLen(req.Title); n < 1 || n > 160 {
		fields["title"] = "Use 1 to 160 characters."
	}
	if runeLen(req.Goal) > 1000 {
		fields["goal"] = "Use at most 1000 characters."
	}
	if runeLen(req.EvidenceRequired) > 1000 {
		fields["evidence_required"] = "Use at most 1000 characters."
	}
	if len(fields) > 0 {
		writeValidation(w, r, fields)
		return
	}

	m, err := s.projects.AddMilestone(r.Context(), u.ID, id, projects.NewMilestone{
		Title: req.Title, Goal: req.Goal, EvidenceRequired: req.EvidenceRequired,
	})
	if errors.Is(err, projects.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "Project not found.")
		return
	}
	if errors.Is(err, projects.ErrTooManyMilestones) {
		writeError(w, r, http.StatusConflict, "milestone_limit", "A project can have at most 20 milestones.")
		return
	}
	if err != nil {
		s.serverError(w, r, "create milestone failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, m)
}
