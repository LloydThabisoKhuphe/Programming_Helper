package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/auth"
	"github.com/LloydThabisoKhuphe/Programming_Helper/backend/internal/submissions"
)

type createSubmissionRequest struct {
	MilestoneID *string                 `json:"milestone_id"`
	Note        string                  `json:"note"`
	Files       []submissions.FileInput `json:"files"`
}

func (s *Server) createSubmission(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	projectID := r.PathValue("id")
	if !uuidRe.MatchString(projectID) {
		writeError(w, r, http.StatusNotFound, "not_found", "Project not found.")
		return
	}

	var req createSubmissionRequest
	if !decodeJSON(w, r, &req, 256*1024) {
		return
	}

	fields := map[string]string{}

	var milestoneID *string
	if req.MilestoneID != nil {
		m := strings.TrimSpace(*req.MilestoneID)
		if m != "" {
			if !uuidRe.MatchString(m) {
				fields["milestone_id"] = "Not a valid milestone ID."
			} else {
				milestoneID = &m
			}
		}
	}

	prepared, problems := submissions.Prepare(req.Files, req.Note)
	for k, v := range problems {
		fields[k] = v
	}
	if len(fields) > 0 {
		writeValidation(w, r, fields)
		return
	}

	sub, created, err := s.submissions.Create(r.Context(), u.ID, projectID, milestoneID, prepared)
	if errors.Is(err, submissions.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "Project not found.")
		return
	}
	if errors.Is(err, submissions.ErrMilestoneNotFound) {
		writeValidation(w, r, map[string]string{"milestone_id": "Milestone not found in this project."})
		return
	}
	if err != nil {
		s.serverError(w, r, "create submission failed", err)
		return
	}

	// 201 for a new version, 200 when the identical content was already the latest version.
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	writeJSON(w, status, sub)
}

func (s *Server) listSubmissions(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	projectID := r.PathValue("id")
	if !uuidRe.MatchString(projectID) {
		writeError(w, r, http.StatusNotFound, "not_found", "Project not found.")
		return
	}

	list, err := s.submissions.List(r.Context(), u.ID, projectID)
	if errors.Is(err, submissions.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "Project not found.")
		return
	}
	if err != nil {
		s.serverError(w, r, "list submissions failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"submissions": list})
}

func (s *Server) getSubmission(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r.Context())
	projectID := r.PathValue("id")
	version, convErr := strconv.Atoi(r.PathValue("version"))
	if !uuidRe.MatchString(projectID) || convErr != nil || version < 1 {
		writeError(w, r, http.StatusNotFound, "not_found", "Submission not found.")
		return
	}

	sub, err := s.submissions.Get(r.Context(), u.ID, projectID, version)
	if errors.Is(err, submissions.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "not_found", "Submission not found.")
		return
	}
	if err != nil {
		s.serverError(w, r, "get submission failed", err)
		return
	}
	writeJSON(w, http.StatusOK, sub)
}
