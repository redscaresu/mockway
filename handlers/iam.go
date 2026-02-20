package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (app *Application) CreateIAMApplication(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateIAMApplication(body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetIAMApplication(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetIAMApplication(chi.URLParam(r, "application_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListIAMApplications(w http.ResponseWriter, _ *http.Request) {
	items, err := app.repo.ListIAMApplications()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "applications", items)
}

func (app *Application) DeleteIAMApplication(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteIAMApplication(chi.URLParam(r, "application_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreateIAMAPIKey(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}

	appID := strings.TrimSpace(anyString(body["application_id"]))
	userID := strings.TrimSpace(anyString(body["user_id"]))
	if (appID == "" && userID == "") || (appID != "" && userID != "") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "either application_id or user_id must be provided (mutually exclusive)", "type": "invalid_argument"})
		return
	}
	if appID == "" {
		delete(body, "application_id")
	}

	out, err := app.repo.CreateIAMAPIKey(body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetIAMAPIKey(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetIAMAPIKey(chi.URLParam(r, "access_key"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListIAMAPIKeys(w http.ResponseWriter, _ *http.Request) {
	items, err := app.repo.ListIAMAPIKeys()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "api_keys", items)
}

func (app *Application) DeleteIAMAPIKey(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteIAMAPIKey(chi.URLParam(r, "access_key")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreateIAMPolicy(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	if strings.TrimSpace(anyString(body["application_id"])) == "" {
		delete(body, "application_id")
	}
	out, err := app.repo.CreateIAMPolicy(body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetIAMPolicy(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetIAMPolicy(chi.URLParam(r, "policy_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListIAMPolicies(w http.ResponseWriter, _ *http.Request) {
	items, err := app.repo.ListIAMPolicies()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "policies", items)
}

func (app *Application) DeleteIAMPolicy(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteIAMPolicy(chi.URLParam(r, "policy_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreateIAMSSHKey(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateIAMSSHKey(body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetIAMSSHKey(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetIAMSSHKey(chi.URLParam(r, "ssh_key_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListIAMSSHKeys(w http.ResponseWriter, _ *http.Request) {
	items, err := app.repo.ListIAMSSHKeys()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "ssh_keys", items)
}

func (app *Application) DeleteIAMSSHKey(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteIAMSSHKey(chi.URLParam(r, "ssh_key_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func anyString(v any) string {
	s, _ := v.(string)
	return s
}
