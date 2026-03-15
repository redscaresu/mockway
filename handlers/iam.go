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

func (app *Application) UpdateIAMApplication(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateIAMApplication(chi.URLParam(r, "application_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
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

func (app *Application) UpdateIAMAPIKey(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateIAMAPIKey(chi.URLParam(r, "access_key"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
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

func (app *Application) UpdateIAMPolicy(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateIAMPolicy(chi.URLParam(r, "policy_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteIAMPolicy(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteIAMPolicy(chi.URLParam(r, "policy_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) SetIAMRules(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	policyID, _ := body["policy_id"].(string)
	rules, _ := body["rules"].([]any)
	if rules == nil {
		rules = []any{}
	}
	result, err := app.repo.SetIAMRules(policyID, rules)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": result, "total_count": len(result)})
}

func (app *Application) CreateIAMRule(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateIAMRule(body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListIAMRules(w http.ResponseWriter, r *http.Request) {
	policyID := r.URL.Query().Get("policy_id")
	if policyID == "" {
		writeList(w, "rules", []map[string]any{})
		return
	}
	items, err := app.repo.ListIAMRulesByPolicy(policyID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "rules", items)
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

func (app *Application) UpdateIAMSSHKey(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateIAMSSHKey(chi.URLParam(r, "ssh_key_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
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

// --- IAM Users ---

func (app *Application) CreateIAMUser(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateIAMUser(body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetIAMUser(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetIAMUser(chi.URLParam(r, "user_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListIAMUsers(w http.ResponseWriter, _ *http.Request) {
	items, err := app.repo.ListIAMUsers()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "users", items)
}

func (app *Application) UpdateIAMUser(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateIAMUser(chi.URLParam(r, "user_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) UpdateIAMUserUsername(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	username, _ := body["username"].(string)
	out, err := app.repo.UpdateIAMUserUsername(chi.URLParam(r, "user_id"), username)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteIAMUser(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteIAMUser(chi.URLParam(r, "user_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

// --- IAM Groups ---

func (app *Application) CreateIAMGroup(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateIAMGroup(body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetIAMGroup(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetIAMGroup(chi.URLParam(r, "group_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListIAMGroups(w http.ResponseWriter, _ *http.Request) {
	items, err := app.repo.ListIAMGroups()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "groups", items)
}

func (app *Application) UpdateIAMGroup(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateIAMGroup(chi.URLParam(r, "group_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteIAMGroup(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteIAMGroup(chi.URLParam(r, "group_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) AddIAMGroupMember(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	userID, _ := body["user_id"].(string)
	out, err := app.repo.AddIAMGroupMember(chi.URLParam(r, "group_id"), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) RemoveIAMGroupMember(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	userID, _ := body["user_id"].(string)
	out, err := app.repo.RemoveIAMGroupMember(chi.URLParam(r, "group_id"), userID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) SetIAMGroupMembers(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	rawIDs, _ := body["user_ids"].([]any)
	userIDs := make([]string, 0, len(rawIDs))
	for _, v := range rawIDs {
		if s, ok := v.(string); ok {
			userIDs = append(userIDs, s)
		}
	}
	out, err := app.repo.SetIAMGroupMembers(chi.URLParam(r, "group_id"), userIDs)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
