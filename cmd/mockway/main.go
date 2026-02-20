package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redscaresu/mockway/handlers"
	"github.com/redscaresu/mockway/repository"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	port := flag.Int("port", 8080, "HTTP port")
	dbPath := flag.String("db", ":memory:", "SQLite database path")
	echoOnly := flag.Bool("echo", false, "Run catch-all echo server for provider path discovery")
	flag.Parse()

	if *echoOnly {
		return runEcho(*port)
	}

	repo, err := repository.New(*dbPath)
	if err != nil {
		return err
	}
	defer repo.Close()

	app := handlers.NewApplication(repo)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	app.RegisterRoutes(r)
	r.NotFound(handlers.UnimplementedHandler)
	r.MethodNotAllowed(handlers.UnimplementedHandler)

	return http.ListenAndServe(fmt.Sprintf(":%d", *port), r)
}

func runEcho(port int) error {
	r := chi.NewRouter()
	r.MethodFunc(http.MethodGet, "/*", logRequestAndOK)
	r.MethodFunc(http.MethodPost, "/*", logRequestAndOK)
	r.MethodFunc(http.MethodPut, "/*", logRequestAndOK)
	r.MethodFunc(http.MethodPatch, "/*", logRequestAndOK)
	r.MethodFunc(http.MethodDelete, "/*", logRequestAndOK)
	r.MethodFunc(http.MethodHead, "/*", logRequestAndOK)
	r.MethodFunc(http.MethodOptions, "/*", logRequestAndOK)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), r)
}

func logRequestAndOK(w http.ResponseWriter, r *http.Request) {
	keys := make([]string, 0, len(r.Header))
	for k := range r.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	log.Printf("[echo] %s %s", r.Method, r.URL.Path)
	for _, k := range keys {
		log.Printf("[echo] header %s=%q", k, r.Header.Get(k))
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}
