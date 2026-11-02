package landing

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func load(t *testing.T) *Site {
	t.Helper()
	s, err := LoadFS(os.DirFS(".."), DefaultPath)
	if err != nil {
		t.Fatalf("committed conf/landing.json must load and validate: %v", err)
	}
	return s
}

func TestCommittedContentLoads(t *testing.T) {
	s := load(t)
	if len(s.FAQ.Items) < 4 || len(s.FAQ.Items) > 6 {
		t.Errorf("want 4-6 FAQ items, got %d", len(s.FAQ.Items))
	}
}

func TestGeneratedOutputs(t *testing.T) {
	s := load(t)
	for _, v := range []obj{s.AppSchema(), s.FAQSchema()} {
		if _, err := json.Marshal(v); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(s.RobotsTxt(), "Sitemap: "+s.URL+"sitemap.xml") || !strings.Contains(s.RobotsTxt(), "Disallow: /api/") {
		t.Errorf("robots: %s", s.RobotsTxt())
	}
	if !strings.Contains(s.SitemapXML(), "<loc>"+s.URL+"</loc>") {
		t.Errorf("sitemap: %s", s.SitemapXML())
	}
	css := s.ThemeCSS()
	for _, want := range []string{"--primary:#4F46E5", "--font-heading:system-ui"} {
		if !strings.Contains(css, want) {
			t.Errorf("theme css missing %q", want)
		}
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	s := load(t)
	s.Title = strings.Repeat("x", 61)
	s.Description = strings.Repeat("y", 161)
	s.URL = "http://example.com"
	s.Theme.Primary = "blue"
	s.FAQ.Items = nil
	s.Features.Screenshots = []Image{{Src: "/static/landing/img/missing.webp", Alt: "x", Width: 1, Height: 1}}
	err := s.Validate(os.DirFS(".."))
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, want := range []string{"title is 61", "description is 161", "url must be", "theme.primary", "faq.items", "missing.webp"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%v", want, err)
		}
	}
}

func TestLoadRejectsUnknownFieldsAndBadJSON(t *testing.T) {
	for name, data := range map[string]string{"unknown": `{"nope":1}`, "bad": `{`} {
		if _, err := LoadFS(fstest.MapFS{"c.json": {Data: []byte(data)}}, "c.json"); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := LoadFS(fstest.MapFS{}, "c.json"); err == nil {
		t.Error("missing file: expected error")
	}
}
