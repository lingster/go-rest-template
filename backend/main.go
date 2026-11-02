// Command server runs the Go REST template API.
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

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
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	container.Version = version
	log.Printf("go-rest-template %s (commit %s, built %s)", version, commit, date)
	cfg, err := config.Load()
	if err != nil {
		return err
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
