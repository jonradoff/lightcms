package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/jonradoff/lightcms/v7/internal/auth"
	"github.com/jonradoff/lightcms/v7/internal/models"
)

// Audit logging for the admin UI.
//
// A /cm route that changes something names its audit action in AdminRoutes
// (.audited("template.update")), and guardAdminRoute writes the entry after
// the handler has run: the action, the signed-in user, the resource id. The
// handlers do not call the audit service themselves, so a new route cannot
// forget to — TestAdminRoutes_EveryWriteRouteIsAudited fails for a non-GET
// route that has neither an action nor an entry in writeRoutesNotAudited.
//
// The entry is written when the handler succeeded, which the wrapper reads
// from the response:
//   - a status of 400 or above is a failure;
//   - a redirect to the login page, or to a URL carrying an error= parameter
//     (how the editor reports a duplicate slug), is a failure;
//   - a form re-rendered with an "Error" (renderAdmin marks it) is a failure;
//   - a handler can say so itself with auditSkip(r) when nothing changed.
//
// The resource id is the route's {id} variable. A create route has none in
// its URL, so its handler passes the new id with auditResource(r, id).

// adminAudit is the per-request record the wrapper hands the handler.
type adminAudit struct {
	resourceID string
	details    map[string]interface{}
	skip       bool
}

type adminAuditKey struct{}

func adminAuditFrom(r *http.Request) *adminAudit {
	a, _ := r.Context().Value(adminAuditKey{}).(*adminAudit)
	return a
}

// auditResource records the id of the resource the request created or
// changed, for routes whose URL does not carry it. Outside an audited route
// it does nothing.
func auditResource(r *http.Request, id string) {
	if a := adminAuditFrom(r); a != nil {
		a.resourceID = id
	}
}

// auditDetail adds a detail to the request's audit entry.
func auditDetail(r *http.Request, key string, value interface{}) {
	if a := adminAuditFrom(r); a != nil {
		if a.details == nil {
			a.details = map[string]interface{}{}
		}
		a.details[key] = value
	}
}

// auditSkip marks the request as having changed nothing: no entry is written.
func auditSkip(r *http.Request) {
	if a := adminAuditFrom(r); a != nil {
		a.skip = true
	}
}

// auditStatusWriter remembers the status the handler answered with.
type auditStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *auditStatusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *auditStatusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *auditStatusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// auditSucceeded reports whether a response is that of a request that went
// through (see the rules at the top of this file).
func auditSucceeded(status int, location string) bool {
	if status == 0 {
		status = http.StatusOK
	}
	if status >= 400 {
		return false
	}
	if status >= 300 {
		if location == "/cm/login" || strings.Contains(location, "error=") {
			return false
		}
	}
	return true
}

// withAdminAudit runs a route's handler and writes its audit entry.
func (h *Handler) withAdminAudit(rt AdminRoute, user *auth.SessionUser, w http.ResponseWriter, r *http.Request) {
	rec := &adminAudit{}
	r = r.WithContext(context.WithValue(r.Context(), adminAuditKey{}, rec))
	sw := &auditStatusWriter{ResponseWriter: w}

	rt.handler(sw, r)

	if rec.skip || h.auditService == nil || !auditSucceeded(sw.status, sw.Header().Get("Location")) {
		return
	}
	entry := models.AuditLog{
		Action:     rt.Audit,
		Resource:   strings.SplitN(rt.Audit, ".", 2)[0],
		ResourceID: rec.resourceID,
		Details:    rec.details,
		UserEmail:  user.Email,
	}
	if oid, err := primitive.ObjectIDFromHex(user.ID); err == nil {
		entry.UserID = oid
	}
	// The route's variables: {id} is the resource, the rest are details
	// (the version reverted to, the page removed from a fork, ...).
	for k, v := range mux.Vars(r) {
		if k == "id" {
			if entry.ResourceID == "" {
				entry.ResourceID = v
			}
			continue
		}
		if entry.Details == nil {
			entry.Details = map[string]interface{}{}
		}
		if _, set := entry.Details[k]; !set {
			entry.Details[k] = v
		}
	}
	h.auditService.LogAsync(entry)
}
