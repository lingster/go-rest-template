package main

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/lingster/go-rest-template/backend/container"
)

const landingTitle = "SaaS Starter – Your app's one-line pitch"

func landingGet(t *testing.T, path string) (int, http.Header, string) {
	t.Helper()
	r := call(t, "GET", path, nil, "")
	return r.Status, r.Header, string(r.Raw)
}

func TestLandingPage(t *testing.T) {
	code, h, body := landingGet(t, "/")
	if code != 200 || !strings.HasPrefix(h.Get("Content-Type"), "text/html") {
		t.Fatalf("GET /: %d %s", code, h.Get("Content-Type"))
	}
	for _, want := range []string{
		"<title>" + strings.NewReplacer("&", "&amp;", "'", "&#39;").Replace(landingTitle) + "</title>",
		`<link rel="canonical" href="https://example.com/">`,
		`lang="en-US"`, `name="theme-color" content="#1E1B4B"`, `property="og:image"`, `name="twitter:card"`,
		"Coming soon to the App Store and Google Play",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("landing page missing %q", want)
		}
	}
	if n := strings.Count(body, "<h1"); n != 1 {
		t.Errorf("want exactly one h1, got %d", n)
	}
	if got := len([]rune(landingTitle)); got > 60 {
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
	if strings.Contains(body, "hero-shot") || strings.Contains(body, `class="shots"`) {
		t.Error("empty hero image / screenshots must be hidden")
	}
	if strings.Contains(body, "<script>") || strings.Contains(body, " style=") {
		t.Error("landing page must not need inline scripts or style attributes (CSP)")
	}
}

func TestLandingJSONLD(t *testing.T) {
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
	if app == nil || app["operatingSystem"] != "iOS, Android" || app["applicationCategory"] != "BusinessApplication" {
		t.Errorf("bad MobileApplication: %v", app)
	}
	if offers := app["offers"].(map[string]any); offers["price"] != "0" || offers["priceCurrency"] != "USD" {
		t.Errorf("bad offers: %v", offers)
	}
	if pub := app["publisher"].(map[string]any); pub["name"] != "Ling LI" || pub["email"] != "ling@aidraw.ai" {
		t.Errorf("bad publisher: %v", pub)
	}
	faq := types["FAQPage"]
	if faq == nil {
		t.Fatal("missing FAQPage")
	}
	for _, q := range faq["mainEntity"].([]any) {
		name := q.(map[string]any)["name"].(string)
		if !strings.Contains(body, "<summary>"+strings.ReplaceAll(name, "'", "&#39;")+"</summary>") {
			t.Errorf("FAQ %q is not visible on the page", name)
		}
	}
}

func TestLandingStoreLinksSwitch(t *testing.T) {
	cfg := container.Get().Cfg
	defer func(old bool) { cfg.LandingStoreLinksLive = old }(cfg.LandingStoreLinksLive)
	links := []string{"https://apps.apple.com/app/id0000000000", "https://play.google.com/store/apps/details?id=com.example.saas_app"}

	cfg.LandingStoreLinksLive = false
	_, _, body := landingGet(t, "/")
	for _, l := range links {
		if strings.Contains(body, l) {
			t.Errorf("store link %s shown while switch is off", l)
		}
	}
	cfg.LandingStoreLinksLive = true
	_, _, body = landingGet(t, "/")
	for _, l := range append(links, "Download on the App Store", "Get it on Google Play") {
		if !strings.Contains(body, strings.ReplaceAll(l, "&", "&amp;")) {
			t.Errorf("missing %q while switch is on", l)
		}
	}
	if strings.Contains(body, "Coming soon") {
		t.Error("coming-soon notice shown while switch is on")
	}
}

func TestRobotsAndSitemap(t *testing.T) {
	code, h, body := landingGet(t, "/robots.txt")
	if code != 200 || !strings.HasPrefix(h.Get("Content-Type"), "text/plain") {
		t.Fatalf("robots: %d %s", code, h.Get("Content-Type"))
	}
	for _, want := range []string{"Allow: /\n", "Disallow: /api/", "Disallow: /admin-ui", "Disallow: /swagger", "Disallow: /healthz",
		"Sitemap: https://example.com/sitemap.xml"} {
		if !strings.Contains(body, want) {
			t.Errorf("robots.txt missing %q", want)
		}
	}
	code, h, body = landingGet(t, "/sitemap.xml")
	if code != 200 || !strings.HasPrefix(h.Get("Content-Type"), "application/xml") {
		t.Fatalf("sitemap: %d %s", code, h.Get("Content-Type"))
	}
	if !strings.Contains(body, "<loc>https://example.com/</loc>") || !strings.Contains(body, "<lastmod>") {
		t.Errorf("sitemap body: %s", body)
	}
}

func TestLandingAssetsAndAPIUnaffected(t *testing.T) {
	code, h, _ := landingGet(t, "/static/landing/landing.css")
	if code != 200 || !strings.Contains(h.Get("Cache-Control"), "max-age") {
		t.Errorf("css: %d %q", code, h.Get("Cache-Control"))
	}
	code, h, body := landingGet(t, "/static/landing/theme.css")
	if code != 200 || !strings.HasPrefix(h.Get("Content-Type"), "text/css") || !strings.Contains(body, "--primary:#4F46E5") {
		t.Errorf("theme.css: %d %s", code, h.Get("Content-Type"))
	}
	if code, _, _ = landingGet(t, "/static/landing/img/og.png"); code != 200 {
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
