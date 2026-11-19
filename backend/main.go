// Command server runs the Go REST template API.
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lingster/go-rest-template/backend/app"
	"github.com/lingster/go-rest-template/backend/config"
	"github.com/lingster/go-rest-template/backend/container"
)

// Set by GoReleaser via -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// assetsFS embeds the generated swagger spec, Swagger UI, admin UI templates, landing page content (conf/landing.json) and static files so the
// binary is self-contained.
//
//go:embed swagger/swagger.json swagger/swagger.yml static views conf
var assetsFS embed.FS

func main() {
	// "server healthcheck" probes the local API: distroless images have no shell or curl for a
	// container HEALTHCHECK.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func healthcheck() int {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		log.Printf("healthcheck: %v", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("healthcheck: status %d", resp.StatusCode)
		return 1
	}
	return 0
}

func run() error {
	container.Version = version
	log.Printf("go-rest-template %s (commit %s, built %s)", version, commit, date)
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	for _, w := range cfg.Warnings() {
		log.Printf("WARNING: %s", w)
	}
	var assets fs.FS = assetsFS
	a, err := app.New(cfg, app.Options{Assets: assets})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return a.Run(ctx)
}
