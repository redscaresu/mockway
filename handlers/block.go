package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/redscaresu/mockway/models"
)

func (app *Application) CreateBlockVolumeHandler(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateBlockVolume(chi.URLParam(r, "zone"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetBlockVolumeHandler(w http.ResponseWriter, r *http.Request) {
	volumeID := chi.URLParam(r, "volume_id")
	out, err := app.repo.GetBlockVolume(volumeID)
	if err == nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	// Fall back to instance volumes for backward compatibility.
	instVol, err2 := app.repo.GetInstanceVolume(chi.URLParam(r, "zone"), volumeID)
	if err2 != nil {
		writeDomainError(w, err2)
		return
	}
	writeJSON(w, http.StatusOK, instVol)
}

func (app *Application) ListBlockVolumes(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListBlockVolumes(chi.URLParam(r, "zone"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "volumes", items)
}

func (app *Application) UpdateBlockVolumeHandler(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateBlockVolume(chi.URLParam(r, "volume_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteBlockVolumeHandler(w http.ResponseWriter, r *http.Request) {
	volumeID := chi.URLParam(r, "volume_id")
	if err := app.repo.DeleteBlockVolume(volumeID); err != nil {
		if !errors.Is(err, models.ErrNotFound) {
			writeDomainError(w, err)
			return
		}
		// Not in block_volumes — try standalone instance volumes, then embedded server volumes.
		if err2 := app.repo.DeleteStandaloneVolume(volumeID); err2 != nil {
			if !errors.Is(err2, models.ErrNotFound) {
				writeDomainError(w, err2)
				return
			}
			if err3 := app.repo.DeleteInstanceVolume(chi.URLParam(r, "zone"), volumeID); err3 != nil {
				writeDomainError(w, err3)
				return
			}
		}
	}
	writeNoContent(w)
}

func (app *Application) CreateBlockSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	volumeID, _ := body["volume_id"].(string)
	out, err := app.repo.CreateBlockSnapshot(chi.URLParam(r, "zone"), volumeID, body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetBlockSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetBlockSnapshot(chi.URLParam(r, "snapshot_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListBlockSnapshots(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListBlockSnapshots(chi.URLParam(r, "zone"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "snapshots", items)
}

func (app *Application) UpdateBlockSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateBlockSnapshot(chi.URLParam(r, "snapshot_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteBlockSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteBlockSnapshot(chi.URLParam(r, "snapshot_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) ListBlockVolumeTypes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"volume_types": []any{
			map[string]any{"type": "sbs_5k", "storage_class": "sbs", "available_sizes": []any{float64(5000000000), float64(10000000000), float64(50000000000)}},
			map[string]any{"type": "sbs_15k", "storage_class": "sbs", "available_sizes": []any{float64(5000000000), float64(10000000000), float64(50000000000)}},
			map[string]any{"type": "l_ssd", "storage_class": "l_ssd", "available_sizes": []any{float64(5000000000), float64(20000000000)}},
		},
		"total_count": 3,
	})
}
