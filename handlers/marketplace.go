package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redscaresu/mockway/models"
)

var marketplaceLabels = []string{
	"ubuntu_noble",
	"ubuntu_jammy",
	"debian_bookworm",
	"centos_stream_9",
}

var marketplaceZones = []string{
	"fr-par-1",
	"nl-ams-1",
	"pl-waw-1",
}

var marketplaceTypes = []string{
	"instance_sbs",
	"instance_local",
}

var compatibleCommercialTypes = []any{
	"DEV1-S", "DEV1-M", "DEV1-L", "GP1-XS", "GP1-S", "GP1-M",
}

func (app *Application) ListMarketplaceLocalImages(w http.ResponseWriter, r *http.Request) {
	imageLabel := strings.TrimSpace(r.URL.Query().Get("image_label"))
	zone := strings.TrimSpace(r.URL.Query().Get("zone"))
	imageType := strings.TrimSpace(r.URL.Query().Get("type"))

	out := make([]map[string]any, 0)
	for _, label := range marketplaceLabels {
		if imageLabel != "" && label != imageLabel {
			continue
		}
		for _, z := range marketplaceZones {
			if zone != "" && z != zone {
				continue
			}
			for _, t := range marketplaceTypes {
				if imageType != "" && t != imageType {
					continue
				}
				out = append(out, map[string]any{
					"id":                          localImageID(label, z, t),
					"compatible_commercial_types": compatibleCommercialTypes,
					"arch":                        "x86_64",
					"zone":                        z,
					"label":                       label,
					"type":                        t,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"local_images": out,
		"total_count":  len(out),
	})
}

func (app *Application) GetMarketplaceLocalImage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "local_image_id")
	for _, label := range marketplaceLabels {
		for _, z := range marketplaceZones {
			for _, t := range marketplaceTypes {
				if id != localImageID(label, z, t) {
					continue
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"id":                          id,
					"compatible_commercial_types": compatibleCommercialTypes,
					"arch":                        "x86_64",
					"zone":                        z,
					"label":                       label,
					"type":                        t,
				})
				return
			}
		}
	}
	writeDomainError(w, models.ErrNotFound)
}

func localImageID(label, zone, imageType string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(label+"|"+zone+"|"+imageType)).String()
}
