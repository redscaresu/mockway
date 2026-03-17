package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (app *Application) CreateVPC(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateVPC(chi.URLParam(r, "region"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetVPC(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetVPC(chi.URLParam(r, "vpc_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListVPCs(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListVPCs(chi.URLParam(r, "region"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "vpcs", items)
}

func (app *Application) UpdateVPC(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateVPC(chi.URLParam(r, "vpc_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteVPC(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteVPC(chi.URLParam(r, "vpc_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

func (app *Application) CreatePrivateNetwork(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreatePrivateNetwork(chi.URLParam(r, "region"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetPrivateNetwork(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetPrivateNetwork(chi.URLParam(r, "pn_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListPrivateNetworks(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListPrivateNetworks(chi.URLParam(r, "region"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "private_networks", items)
}

func (app *Application) UpdatePrivateNetwork(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdatePrivateNetwork(chi.URLParam(r, "pn_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeletePrivateNetwork(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeletePrivateNetwork(chi.URLParam(r, "pn_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

// --- VPC Routes ---

func (app *Application) CreateVPCRoute(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateVPCRoute(chi.URLParam(r, "region"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetVPCRoute(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetVPCRoute(chi.URLParam(r, "route_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListVPCRoutes(w http.ResponseWriter, r *http.Request) {
	vpcID := r.URL.Query().Get("vpc_id")
	items, err := app.repo.ListVPCRoutes(chi.URLParam(r, "region"), vpcID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "routes", items)
}

func (app *Application) UpdateVPCRoute(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateVPCRoute(chi.URLParam(r, "route_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteVPCRoute(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteVPCRoute(chi.URLParam(r, "route_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

// --- VPC Public Gateways ---

func (app *Application) CreateVPCPublicGateway(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.CreateVPCPublicGateway(chi.URLParam(r, "zone"), body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetVPCPublicGateway(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetVPCPublicGateway(chi.URLParam(r, "gateway_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListVPCPublicGateways(w http.ResponseWriter, r *http.Request) {
	items, err := app.repo.ListVPCPublicGateways(chi.URLParam(r, "zone"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeList(w, "gateways", items)
}

func (app *Application) UpdateVPCPublicGateway(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateVPCPublicGateway(chi.URLParam(r, "gateway_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteVPCPublicGateway(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteVPCPublicGateway(chi.URLParam(r, "gateway_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}

// --- VPC Gateway Networks ---

func (app *Application) CreateVPCGatewayNetwork(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	body["zone"] = chi.URLParam(r, "zone")
	out, err := app.repo.CreateVPCGatewayNetwork(body)
	if err != nil {
		writeCreateError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) GetVPCGatewayNetwork(w http.ResponseWriter, r *http.Request) {
	out, err := app.repo.GetVPCGatewayNetwork(chi.URLParam(r, "gateway_network_id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) ListVPCGatewayNetworks(w http.ResponseWriter, r *http.Request) {
	gatewayID := r.URL.Query().Get("gateway_id")
	zone := chi.URLParam(r, "zone")
	items, err := app.repo.ListVPCGatewayNetworks(gatewayID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// Filter by zone — the endpoint is zone-scoped.
	if zone != "" {
		filtered := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if z, _ := item["zone"].(string); z == zone {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	writeList(w, "gateway_networks", items)
}

func (app *Application) UpdateVPCGatewayNetwork(w http.ResponseWriter, r *http.Request) {
	body, err := decodeBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid json", "type": "invalid_argument"})
		return
	}
	out, err := app.repo.UpdateVPCGatewayNetwork(chi.URLParam(r, "gateway_network_id"), body)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (app *Application) DeleteVPCGatewayNetwork(w http.ResponseWriter, r *http.Request) {
	if err := app.repo.DeleteVPCGatewayNetwork(chi.URLParam(r, "gateway_network_id")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeNoContent(w)
}
