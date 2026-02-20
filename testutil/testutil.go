package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/redscaresu/mockway/handlers"
	"github.com/redscaresu/mockway/repository"
)

func NewTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()
	repo, err := repository.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	app := handlers.NewApplication(repo)
	r := chi.NewRouter()
	app.RegisterRoutes(r)
	r.NotFound(handlers.UnimplementedHandler)
	r.MethodNotAllowed(handlers.UnimplementedHandler)
	ts := httptest.NewServer(r)
	cleanup := func() {
		ts.Close()
		_ = repo.Close()
	}
	return ts, cleanup
}

func DoCreate(t *testing.T, ts *httptest.Server, path string, body any) (int, map[string]any) {
	t.Helper()
	return doJSON(t, ts, http.MethodPost, path, body)
}

func DoGet(t *testing.T, ts *httptest.Server, path string) (int, map[string]any) {
	t.Helper()
	return doJSON(t, ts, http.MethodGet, path, nil)
}

func DoList(t *testing.T, ts *httptest.Server, path string) (int, map[string]any) {
	t.Helper()
	return doJSON(t, ts, http.MethodGet, path, nil)
}

func DoDelete(t *testing.T, ts *httptest.Server, path string) int {
	t.Helper()
	status, _ := doJSON(t, ts, http.MethodDelete, path, nil)
	return status
}

func ResetState(t *testing.T, ts *httptest.Server) {
	t.Helper()
	status, _ := doJSON(t, ts, http.MethodPost, "/mock/reset", nil)
	if status != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", status)
	}
}

func GetState(t *testing.T, ts *httptest.Server) map[string]any {
	t.Helper()
	status, body := doJSON(t, ts, http.MethodGet, "/mock/state", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	return body
}

func doJSON(t *testing.T, ts *httptest.Server, method, path string, payload any) (int, map[string]any) {
	t.Helper()
	var bodyBytes []byte
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		bodyBytes = b
	}

	req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if !strings.HasPrefix(path, "/mock/") {
		req.Header.Set("X-Auth-Token", "test-token")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return resp.StatusCode, nil
	}

	decoded := map[string]any{}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, decoded
}
