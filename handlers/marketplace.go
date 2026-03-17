package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// knownMarketplaceLabels covers common Scaleway marketplace images.
// The handler also accepts any unknown label dynamically so that new images
// don't require a code change.
var knownMarketplaceLabels = []string{
	// Ubuntu
	"ubuntu_noble", "ubuntu_jammy", "ubuntu_focal",
	// Debian
	"debian_bookworm", "debian_bullseye", "debian_trixie",
	// CentOS / RHEL-family
	"centos_stream_9", "rockylinux_9", "almalinux_9",
	// Fedora
	"fedora_40", "fedora_39",
	// Arch
	"archlinux",
	// Alpine
	"alpine",
}

var marketplaceZones = []string{
	"fr-par-1", "fr-par-2", "fr-par-3",
	"nl-ams-1", "nl-ams-2", "nl-ams-3",
	"pl-waw-1", "pl-waw-2", "pl-waw-3",
}

var marketplaceTypes = []string{
	"instance_sbs",
	"instance_local",
}

var compatibleCommercialTypes = []any{
	"DEV1-S", "DEV1-M", "DEV1-L", "DEV1-XL",
	"GP1-XS", "GP1-S", "GP1-M", "GP1-L", "GP1-XL",
	"PRO2-XXS", "PRO2-XS", "PRO2-S", "PRO2-M", "PRO2-L",
	"PLAY2-PICO", "PLAY2-NANO", "PLAY2-MICRO",
	"STARDUST1-S",
	"ENT1-S", "ENT1-M", "ENT1-L", "ENT1-XL", "ENT1-2XL",
	"POP2-2C-8G", "POP2-4C-16G", "POP2-8C-32G",
}

func (app *Application) ListMarketplaceLocalImages(w http.ResponseWriter, r *http.Request) {
	imageLabel := strings.TrimSpace(r.URL.Query().Get("image_label"))
	zone := strings.TrimSpace(r.URL.Query().Get("zone"))
	imageType := strings.TrimSpace(r.URL.Query().Get("type"))

	// Determine which labels to enumerate. If a specific label is requested,
	// return results for it regardless of whether it's in the known list.
	labels := knownMarketplaceLabels
	if imageLabel != "" {
		labels = []string{imageLabel}
		// Persist the label so GET can resolve its IDs after restart.
		_ = app.repo.AddMarketplaceLabel(imageLabel)
	}

	out := make([]map[string]any, 0)
	for _, label := range labels {
		for _, z := range marketplaceZones {
			if zone != "" && z != zone {
				continue
			}
			for _, t := range marketplaceTypes {
				if imageType != "" && t != imageType {
					continue
				}
				out = append(out, localImageEntry(label, z, t))
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

	// Build label list: known + persisted dynamic labels.
	allLabels := make([]string, 0, len(knownMarketplaceLabels))
	allLabels = append(allLabels, knownMarketplaceLabels...)
	if dynamic, err := app.repo.ListMarketplaceLabels(); err == nil {
		allLabels = append(allLabels, dynamic...)
	}

	for _, label := range allLabels {
		for _, z := range marketplaceZones {
			for _, t := range marketplaceTypes {
				if id == localImageID(label, z, t) {
					writeJSON(w, http.StatusOK, localImageEntry(label, z, t))
					return
				}
			}
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"message": "resource not found", "type": "not_found"})
}

func localImageEntry(label, zone, imageType string) map[string]any {
	return map[string]any{
		"id":                          localImageID(label, zone, imageType),
		"compatible_commercial_types": compatibleCommercialTypes,
		"arch":                        "x86_64",
		"zone":                        zone,
		"label":                       label,
		"type":                        imageType,
	}
}

func localImageID(label, zone, imageType string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(label+"|"+zone+"|"+imageType)).String()
}
