package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/lingster/go-rest-template/backend/testutil"
)

func TestSwaggerIsMountedOnlyWhenEnabled(t *testing.T) {
	assets := fstest.MapFS{
		"swagger/swagger.json":            {Data: []byte(`{"swagger":"2.0"}`)},
		"static/swagger-ui/index.html":    {Data: []byte("<html></html>")},
		"static/admin/app.js":             {Data: []byte("//")},
		"static/landing/landing.css":      {Data: []byte("/**/")},
		"static/swagger-ui/swagger-ui.js": {Data: []byte("//")},
	}
	get := func(enabled bool, path string) int {
		cfg := testutil.Config()
		cfg.SwaggerEnabled = enabled
		rec := httptest.NewRecorder()
		buildHandler(cfg, assets).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec.Code
	}
	for _, p := range []string{"/swagger/", "/swagger/swagger.json", "/swagger/swagger-ui.js"} {
		if got := get(true, p); got != http.StatusOK {
			t.Errorf("enabled %s: %d", p, got)
		}
		if got := get(false, p); got != http.StatusNotFound {
			t.Errorf("disabled %s: %d, want 404", p, got)
		}
	}
	if got := get(false, "/static/admin/app.js"); got != http.StatusOK {
		t.Errorf("disabling swagger must not affect other assets: %d", got)
	}
}
