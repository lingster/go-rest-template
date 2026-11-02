package main

import (
	"encoding/json"
	"html"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/lingster/go-rest-template/backend/container"
	"github.com/lingster/go-rest-template/backend/landing"
)

// The tests read expectations from the loaded conf/landing.json, so they keep passing after the project
// is renamed (scripts/init_project.py rewrites the content) or the copy is edited.
func site(t *testing.T) *landing.Site {
	t.Helper()
	s := landing.Current()
	if s == nil {
		t.Fatal("landing content not loaded")
	}
	return s
}

// esc is how html/template escapes text in element and attribute context.
func esc(s string) string { return html.EscapeString(s) }

func landingGet(t *testing.T, path string) (int, http.Header, string) {
	t.Helper()
	r := call(t, "GET", path, nil, "")
	return r.Status, r.Header, string(r.Raw)
}

func TestLandingPage(t *testing.T) {
	s := site(t)
	code, h, body := landingGet(t, "/")
	if code != 200 || !strings.HasPrefix(h.Get("Content-Type"), "text/html") {
		t.Fatalf("GET /: %d %s", code, h.Get("Content-Type"))
	}
	for _, want := range []string{
		"<title>" + esc(s.Title) + "</title>",
		`<link rel="canonical" href="` + s.URL + `">`,
		`lang="` + s.Locale + `"`, `name="theme-color" content="` + s.Theme.Hero + `"`, `property="og:image"`, `name="twitter:card"`,
		esc(s.Stores.ComingSoon),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("landing page missing %q", want)
		}
	}
	if n := strings.Count(body, "<h1"); n != 1 {
		t.Errorf("want exactly one h1, got %d", n)
	}
	if got := len([]rune(s.Title)); got > 60 {
		t.Errorf("title is %d chars", got)
	}
	if d := regexp.MustCompile(`name="description" content="([^"]*)"`).FindStringSubmatch(body); d == nil || len(d[1]) > 160 {
		t.Errorf("meta description missing or over 160 chars: %v", d)
	}
	if cc := h.Get("Cache-Control"); !strings.Contains(cc, "max-age") || strings.Contains(cc, "no-store") {
		t.Errorf("landing Cache-Control = %q", cc)
	}
	if csp := h.Get("Content-Security-Policy"); !strings.Contains(csp, "style-src 'self'") || !strings.Contains(csp, "font-src 'self'") {
		t.Errorf("CSP must allow same-origin css/fonts: %q", csp)
	}
	if (s.Hero.Image.Src == "") == strings.Contains(body, "hero-shot") {
		t.Error("hero picture must be shown exactly when hero.image.src is set")
	}
	if (len(s.Features.Screenshots) == 0) == strings.Contains(body, `class="shots"`) {
		t.Error("screenshot block must be shown exactly when features.screenshots is not empty")
	}
	if strings.Contains(body, "<script>") || strings.Contains(body, " style=") {
		t.Error("landing page must not need inline scripts or style attributes (CSP)")
	}
}

func TestLandingJSONLD(t *testing.T) {
	s := site(t)
	_, _, body := landingGet(t, "/")
	blocks := regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`).FindAllStringSubmatch(body, -1)
	if len(blocks) != 2 {
		t.Fatalf("want 2 JSON-LD blocks, got %d", len(blocks))
	}
	types := map[string]map[string]any{}
	for _, b := range blocks {
		var v map[string]any
		if err := json.Unmarshal([]byte(b[1]), &v); err != nil {
			t.Fatalf("invalid JSON-LD: %v\n%s", err, b[1])
		}
		types[v["@type"].(string)] = v
	}
	app := types["MobileApplication"]
	if app == nil || app["operatingSystem"] != s.App.OperatingSystem || app["applicationCategory"] != s.App.Category {
		t.Errorf("bad MobileApplication: %v", app)
	}
	if offers := app["offers"].(map[string]any); offers["price"] != s.Pricing.Price || offers["priceCurrency"] != s.Pricing.Currency {
		t.Errorf("bad offers: %v", offers)
	}
	if pub := app["publisher"].(map[string]any); pub["name"] != s.Publisher.Name || pub["email"] != s.Publisher.Email {
		t.Errorf("bad publisher: %v", pub)
	}
	faq := types["FAQPage"]
	if faq == nil {
		t.Fatal("missing FAQPage")
	}
	for _, q := range faq["mainEntity"].([]any) {
		name := q.(map[string]any)["name"].(string)
		if !strings.Contains(body, "<summary>"+esc(name)+"</summary>") {
			t.Errorf("FAQ %q is not visible on the page", name)
		}
	}
}

func TestLandingStoreLinksSwitch(t *testing.T) {
	s := site(t)
	cfg := container.Get().Cfg
	defer func(old bool) { cfg.LandingStoreLinksLive = old }(cfg.LandingStoreLinksLive)
	links := []string{s.Stores.AppStoreURL, s.Stores.PlayStoreURL}

	cfg.LandingStoreLinksLive = false
	_, _, body := landingGet(t, "/")
	for _, l := range links {
		if strings.Contains(body, l) {
			t.Errorf("store link %s shown while switch is off", l)
		}
	}
	cfg.LandingStoreLinksLive = true
	_, _, body = landingGet(t, "/")
	for _, l := range append(links, s.Stores.AppStoreLabel, s.Stores.PlayStoreLabel) {
		if !strings.Contains(body, esc(l)) {
			t.Errorf("missing %q while switch is on", l)
		}
	}
	if strings.Contains(body, esc(s.Stores.ComingSoon)) {
		t.Error("coming-soon notice shown while switch is on")
	}
}

func TestRobotsAndSitemap(t *testing.T) {
	s := site(t)
	code, h, body := landingGet(t, "/robots.txt")
	if code != 200 || !strings.HasPrefix(h.Get("Content-Type"), "text/plain") {
		t.Fatalf("robots: %d %s", code, h.Get("Content-Type"))
	}
	for _, want := range []string{"Allow: /\n", "Disallow: /api/", "Disallow: /admin-ui", "Disallow: /swagger", "Disallow: /healthz",
		"Sitemap: " + s.URL + "sitemap.xml"} {
		if !strings.Contains(body, want) {
			t.Errorf("robots.txt missing %q", want)
		}
	}
	code, h, body = landingGet(t, "/sitemap.xml")
	if code != 200 || !strings.HasPrefix(h.Get("Content-Type"), "application/xml") {
		t.Fatalf("sitemap: %d %s", code, h.Get("Content-Type"))
	}
	if !strings.Contains(body, "<loc>"+s.URL+"</loc>") || !strings.Contains(body, "<lastmod>") {
		t.Errorf("sitemap body: %s", body)
	}
}

func TestLandingAssetsAndAPIUnaffected(t *testing.T) {
	s := site(t)
	code, h, _ := landingGet(t, "/static/landing/landing.css")
	if code != 200 || !strings.Contains(h.Get("Cache-Control"), "max-age") {
		t.Errorf("css: %d %q", code, h.Get("Cache-Control"))
	}
	code, h, body := landingGet(t, "/static/landing/theme.css")
	if code != 200 || !strings.HasPrefix(h.Get("Content-Type"), "text/css") || !strings.Contains(body, "--primary:"+s.Theme.Primary) {
		t.Errorf("theme.css: %d %s", code, h.Get("Content-Type"))
	}
	if code, _, _ = landingGet(t, s.OGImage.Src); code != 200 {
		t.Errorf("og image: %d", code)
	}
	if code, _, _ = landingGet(t, "/swagger/"); code != 200 {
		t.Errorf("swagger: %d", code)
	}
	code, h, body = landingGet(t, "/healthz")
	if code != 200 || !strings.Contains(body, `"ok"`) || h.Get("Content-Security-Policy") != "default-src 'none'; frame-ancestors 'none'" {
		t.Errorf("healthz: %d csp=%q", code, h.Get("Content-Security-Policy"))
	}
	code, _, body = landingGet(t, "/api/v1/me")
	if code != 401 || !strings.Contains(body, `"error"`) {
		t.Errorf("api auth: %d %s", code, body)
	}
	if code, _, _ = landingGet(t, "/nope"); code != 404 {
		t.Errorf("unknown path: %d", code)
	}
}
