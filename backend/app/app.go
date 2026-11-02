// Package app builds the HTTP application: database, services, Beego configuration and middleware.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/beego/beego/v2/server/web"
	beecontext "github.com/beego/beego/v2/server/web/context"

	"github.com/lingster/go-rest-template/backend/apperr"
	"github.com/lingster/go-rest-template/backend/config"
	"github.com/lingster/go-rest-template/backend/container"
	"github.com/lingster/go-rest-template/backend/database"
	"github.com/lingster/go-rest-template/backend/filters"
	"github.com/lingster/go-rest-template/backend/httpx"
	"github.com/lingster/go-rest-template/backend/landing"
	"github.com/lingster/go-rest-template/backend/mail"
	"github.com/lingster/go-rest-template/backend/repositories"
	_ "github.com/lingster/go-rest-template/backend/routers" // registers routes
	"github.com/lingster/go-rest-template/backend/services"
)

// Options customises App construction.
type Options struct {
	// Mailer overrides the configured mail driver (tests).
	Mailer mail.Mailer
	// GoogleVerifier overrides the Google ID token verifier (tests).
	GoogleVerifier services.GoogleTokenVerifier
	// Assets holds the "swagger", "static" and "views" directories (normally //go:embed).
	Assets fs.FS
}

// App is the assembled application.
type App struct {
	Cfg       *config.Config
	Container *container.Container
	handler   http.Handler
}

var beegoOnce sync.Once

// New initialises the database, wires services, seeds the admin and builds the handler.
// Because Beego and its ORM are process-global, New may only be used with one DB per process.
func New(cfg *config.Config, opts Options) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := database.Init(cfg.DBDriver, cfg.DatabaseURL); err != nil {
		return nil, err
	}
	c, err := container.New(cfg, repositories.NewORMSet(), opts.Mailer, container.WithGoogleVerifier(opts.GoogleVerifier))
	if err != nil {
		return nil, err
	}
	container.Set(c)

	var cfgErr error
	beegoOnce.Do(func() { cfgErr = configureBeego(opts.Assets) })
	if cfgErr != nil {
		return nil, cfgErr
	}
	if cfg.AdminEmail != "" && cfg.AdminPassword != "" {
		if err := c.Admin.SeedAdmin(context.Background(), cfg.AdminEmail, cfg.AdminPassword); err != nil {
			return nil, fmt.Errorf("seed admin: %w", err)
		}
	}
	return &App{Cfg: cfg, Container: c, handler: buildHandler(cfg, opts.Assets)}, nil
}

// Handler returns the full http.Handler (middleware + static + Beego router).
func (a *App) Handler() http.Handler { return a.handler }

// Run serves until ctx is cancelled, then shuts down gracefully.
func (a *App) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", a.Cfg.HTTPPort), Handler: a.handler,
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second,
		IdleTimeout: 120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Printf("listening on %s (env=%s db=%s)", srv.Addr, a.Cfg.AppEnv, a.Cfg.DBDriver)
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func buildHandler(cfg *config.Config, assets fs.FS) http.Handler {
	mux := http.NewServeMux()
	if assets != nil {
		mux.Handle("/swagger/", swaggerHandler(assets))
		mux.Handle("/static/admin/", http.StripPrefix("/static/admin/", http.FileServerFS(mustSub(assets, "static/admin"))))
		// theme.css is generated from conf/landing.json by the router; it must win over the file server below.
		mux.Handle("/static/landing/theme.css", web.BeeApp.Handlers)
		// Landing page assets (CSS, images, optional fonts): public and cacheable; "/" itself is a Beego route.
		mux.Handle("/static/landing/", http.StripPrefix("/static/landing/",
			withCacheControl(landingAssetCache, http.FileServerFS(mustSub(assets, "static/landing")))))
	}
	mux.Handle("/", web.BeeApp.Handlers)

	var limiter filters.Limiter
	if cfg.RateLimitPerMinute > 0 {
		limiter = filters.NewFixedWindowLimiter(cfg.RateLimitPerMinute, time.Minute)
	}
	return filters.Chain(mux,
		filters.Recover(),
		filters.RequestLogger(cfg.TrustProxy),
		filters.SecurityHeaders(cfg),
		filters.CORS(cfg.CORSAllowedOrigins),
		filters.RateLimit(limiter, "/api/v1/auth/", cfg.TrustProxy),
		filters.ForceJSONBody("/api/"),
	)
}

// landingAssetCache: file names are not fingerprinted, so keep the lifetime to a day.
const landingAssetCache = "public, max-age=86400"

func withCacheControl(value string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		next.ServeHTTP(w, r)
	})
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

// swaggerHandler serves the generated spec from swagger/ and the UI files from static/swagger-ui/.
func swaggerHandler(assets fs.FS) http.Handler {
	spec := http.FileServerFS(mustSub(assets, "swagger"))
	ui := http.FileServerFS(mustSub(assets, "static/swagger-ui"))
	return http.StripPrefix("/swagger/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "swagger.json", "swagger.yml":
			spec.ServeHTTP(w, r)
		default:
			ui.ServeHTTP(w, r)
		}
	}))
}

func configureBeego(assets fs.FS) error {
	c := web.BConfig
	c.RunMode = web.PROD
	c.RecoverPanic = true
	c.CopyRequestBody = false
	c.Log.AccessLogs = false
	c.WebConfig.AutoRender = false
	c.WebConfig.EnableXSRF = false
	c.RecoverFunc = recoverFunc

	jsonErr := func(err *apperr.Error) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { httpx.WriteError(w, err) }
	}
	web.ErrorHandler("404", jsonErr(apperr.NotFound("Resource not found")))
	web.ErrorHandler("405", jsonErr(apperr.New(http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")))
	web.ErrorHandler("500", jsonErr(apperr.Internal()))

	if assets != nil {
		site, err := landing.LoadFS(assets, landing.DefaultPath)
		if err != nil {
			return err
		}
		landing.Set(site)
		web.SetTemplateFSFunc(func() http.FileSystem { return http.FS(assets) })
		if err := web.AddViewPath("views"); err != nil {
			return fmt.Errorf("load templates: %w", err)
		}
	}
	return nil
}

func recoverFunc(ctx *beecontext.Context, _ *web.Config) {
	if r := recover(); r != nil {
		if errors.Is(asError(r), web.ErrAbort) {
			return
		}
		log.Printf("panic: %v (%s %s)", r, ctx.Request.Method, ctx.Request.URL.Path)
		httpx.WriteError(ctx.ResponseWriter, apperr.Internal())
	}
}

func asError(v interface{}) error {
	if e, ok := v.(error); ok {
		return e
	}
	return nil
}
