package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/redscaresu/mockway/models"
)

func (app *Application) ResetState(w http.ResponseWriter, _ *http.Request) {
	if err := app.repo.Reset(); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) GetState(w http.ResponseWriter, _ *http.Request) {
	state, err := app.repo.FullState()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (app *Application) GetServiceState(w http.ResponseWriter, r *http.Request) {
	service := chi.URLParam(r, "service")
	state, err := app.repo.ServiceState(service)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unknown service", "type": "not_found"})
			return
		}
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}
