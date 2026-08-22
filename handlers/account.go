package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/redscaresu/mockway/models"
)

// CRITICAL[account-project-create-returns-id]: POST /account/v3/projects
// MUST return a body containing a non-empty `id`. The Scaleway provider
// reads it straight into state for scaleway_account_project and every
// child resource interpolates `project_id` from it, so an empty or
// missing id fails the whole graph at plan time on the next run rather
// than at create.
//
// This route exists for infrafactory Layer 3 (ADR-0010): the generated
// HCL must declare a scaleway_account_project, and the SAME HCL is
// applied to mockway first. Without these handlers the call 501s, Layer
// 2 fails, and Layer 3 -- which Layer 2 gates -- never runs at all.
//
// Locked in by TestContract_account_project_create_returns_id.
func (app *Application) CreateAccountProject(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateAccountProject(body)
	if err != nil {
		writeCreateErrorFor(w, err, "project", "")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetAccountProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "project_id")
	out, err := app.repo.GetAccountProject(id)
	if err != nil {
		writeDomainErrorFor(w, err, "project", id)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListAccountProjects(w http.ResponseWriter, _ *http.Request) {
	items, err := app.repo.ListAccountProjects()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "projects", items)
}

func (app *Application) UpdateAccountProject(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	id := chi.URLParam(r, "project_id")
	out, err := app.repo.UpdateAccountProject(id, body)
	if err != nil {
		writeDomainErrorFor(w, err, "project", id)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// CRITICAL[account-project-delete-nonempty]: DELETE on a project that
// still owns resources MUST fail with 409 and name the blocking
// resources, never succeed silently.
//
// Real Scaleway refuses to delete a non-empty project. Mirroring that
// refusal is the whole safety value of modelling projects here: a stack
// whose destroy ordering is wrong then fails in Layer 2 in seconds,
// before it has cost anything, instead of failing at real-Scaleway
// teardown and leaving billable orphans behind. A mock that cheerfully
// deleted a populated project would teach the generator a destroy
// ordering that does not work against the real API -- the exact
// divergence Layer 2 exists to catch.
//
// The blocker list is load-bearing, not decoration: "cannot delete:
// dependents exist" tells an operator nothing about which teardown step
// went wrong.
//
// Locked in by TestContract_account_project_delete_nonempty.
func (app *Application) DeleteAccountProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "project_id")
	err := app.repo.DeleteAccountProject(id)
	if err == nil {
		writeNoContent(w)
		return
	}

	var notEmpty *models.ProjectNotEmptyError
	if errors.As(err, &notEmpty) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"message":    notEmpty.Error(),
			"type":       "precondition_failed",
			"resource":   "project",
			"blockers":   notEmpty.Blockers,
			"project_id": notEmpty.ProjectID,
		})
		return
	}
	writeDomainErrorFor(w, err, "project", id)
}
