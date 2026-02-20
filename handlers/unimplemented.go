package handlers

import (
	"fmt"
	"log"
	"net/http"
)

func UnimplementedHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("UNIMPLEMENTED: %s %s", r.Method, r.URL.String())
	writeJSON(w, http.StatusNotImplemented, map[string]any{
		"message": fmt.Sprintf("not implemented: %s %s", r.Method, r.URL.Path),
		"type":    "not_implemented",
	})
}
