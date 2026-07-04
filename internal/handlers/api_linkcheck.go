package handlers

import (
	"net/http"

	"github.com/jonradoff/lightcms/v7/internal/auth"
	"github.com/jonradoff/lightcms/v7/internal/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SetLinkCheckerService attaches a LinkCheckerService to the APIHandler.
func (a *APIHandler) SetLinkCheckerService(lc *services.LinkCheckerService) {
	a.linkCheckerService = lc
}

// POST /api/v1/link-check — start a new link check job
func (a *APIHandler) APIStartLinkCheck(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentEdit) {
		return
	}
	if a.linkCheckerService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "link checker service unavailable")
		return
	}

	jobID, err := a.linkCheckerService.StartJob(r.Context())
	if err != nil {
		a.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	a.jsonResponse(w, http.StatusAccepted, map[string]interface{}{
		"job_id":  jobID.Hex(),
		"message": "Link check started",
	})
}

// GET /api/v1/link-check/{id} — get job status and results
func (a *APIHandler) APIGetLinkCheckJob(w http.ResponseWriter, r *http.Request) {
	if !a.requirePermission(w, r, auth.PermContentEdit) {
		return
	}
	if a.linkCheckerService == nil {
		a.jsonError(w, http.StatusServiceUnavailable, "link checker service unavailable")
		return
	}

	id, err := primitive.ObjectIDFromHex(mux.Vars(r)["id"])
	if err != nil {
		a.jsonError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	job, err := a.linkCheckerService.GetJob(r.Context(), id)
	if err != nil {
		a.jsonError(w, http.StatusNotFound, "job not found")
		return
	}

	a.jsonResponse(w, http.StatusOK, job)
}
