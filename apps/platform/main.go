package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/bernardoforcillo/tendersbay-xyz/apps/platform/internal/server"
)

//go:embed all:dist
var distFS embed.FS

func main() {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		log.Fatalf("failed to load embedded frontend: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	log.Printf("platform listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, server.New(dist)); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
