// Package landing holds the data-driven public landing page: a typed content file (conf/landing.json),
// its validation, and generators for the JSON-LD, robots.txt, sitemap.xml and theme CSS derived from it.
// Nothing app-specific lives in code or in the template; edit the JSON to rebrand.
package landing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

// DefaultPath is the content file inside the embedded assets.
const DefaultPath = "conf/landing.json"

const (
	maxTitle       = 60
	maxDescription = 160
)

type Image struct {
	Src     string `json:"src"`
	Alt     string `json:"alt"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Caption string `json:"caption,omitempty"`
}

type Item struct {
	Icon  string `json:"icon,omitempty"` // optional short glyph shown before the title
	Title string `json:"title"`
	Text  string `json:"text"`
}

type Section struct {
	Eyebrow string `json:"eyebrow"`
	Heading string `json:"heading"`
	Intro   string `json:"intro,omitempty"`
}

type Theme struct {
	Background string `json:"background"`
	Text       string `json:"text"`
	Muted      string `json:"muted"`
	Primary    string `json:"primary"`
	Accent     string `json:"accent"`
	Hero       string `json:"hero"`
	HeroLabel  string `json:"heroLabel"`
	Card       string `json:"card"`
	Line       string `json:"line"`
}

type FontFace struct {
	Weight int    `json:"weight"`
	File   string `json:"file"`
}

type Font struct {
	Family  string     `json:"family"`
	Faces   []FontFace `json:"faces"`
	Preload string     `json:"preload,omitempty"`
}

type Link struct {
	Label string `json:"label"`
	Href  string `json:"href"`
}

type FAQItem struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// Site is the whole landing page content.
type Site struct {
	Comment      string `json:"_comment,omitempty"` // free text for editors; ignored
	SiteName     string `json:"siteName"`
	URL          string `json:"url"` // canonical, https, trailing slash
	Locale       string `json:"locale"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Tagline      string `json:"tagline"`
	LastModified string `json:"lastModified"` // YYYY-MM-DD, sitemap lastmod
	OGImage      Image  `json:"ogImage"`
	Icons        struct {
		SVG        string `json:"svg"`
		PNG32      string `json:"png32"`
		AppleTouch string `json:"appleTouch"`
	} `json:"icons"`
	Theme Theme `json:"theme"`
	// Fonts is optional: leave a family empty to use the system font stack.
	Fonts struct {
		Heading Font `json:"heading"`
		Body    Font `json:"body"`
	} `json:"fonts"`
	Brand struct {
		Logo Image `json:"logo"`
	} `json:"brand"`
	Hero struct {
		Eyebrow  string `json:"eyebrow"`
		Headline string `json:"headline"`
		Subhead  string `json:"subhead"`
		Note     string `json:"note"`
		Image    Image  `json:"image"` // optional: an empty src hides the hero picture
	} `json:"hero"`
	Features struct {
		Section
		Items       []Item  `json:"items"`
		Screenshots []Image `json:"screenshots"` // optional: an empty list hides the block
	} `json:"features"`
	Steps struct {
		Section
		Items []Item `json:"items"`
	} `json:"steps"`
	Pricing struct {
		Section
		Text     string `json:"text"`
		Price    string `json:"price"`
		Currency string `json:"currency"`
	} `json:"pricing"`
	FAQ struct {
		Section
		Items []FAQItem `json:"items"`
	} `json:"faq"`
	Stores struct {
		Heading        string `json:"heading"`
		AppStoreURL    string `json:"appStoreUrl"`
		AppStoreLabel  string `json:"appStoreLabel"`
		PlayStoreURL   string `json:"playStoreUrl"`
		PlayStoreLabel string `json:"playStoreLabel"`
		ComingSoon     string `json:"comingSoon"`
	} `json:"stores"`
	App struct {
		OperatingSystem string `json:"operatingSystem"`
		Category        string `json:"category"`
	} `json:"app"`
	Publisher struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Email string `json:"email"`
	} `json:"publisher"`
	Footer struct {
		Links      []Link `json:"links"`
		Disclaimer string `json:"disclaimer"`
	} `json:"footer"`
	Robots struct {
		Disallow []string `json:"disallow"`
	} `json:"robots"`
}

// LoadFS reads, decodes (unknown fields are errors) and validates the content file; referenced
// local assets (images, icons, fonts) must exist in fsys.
func LoadFS(fsys fs.FS, path string) (*Site, error) {
	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("landing: read %s: %w", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var s Site
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("landing: parse %s: %w", path, err)
	}
	if err := s.Validate(fsys); err != nil {
		return nil, fmt.Errorf("landing: invalid %s: %w", path, err)
	}
	return &s, nil
}

var (
	hexColour = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	dateRe    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// Validate reports every problem at once. fsys may be nil to skip asset existence checks.
func (s *Site) Validate(fsys fs.FS) error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	req := func(field, v string) {
		if strings.TrimSpace(v) == "" {
			bad("%s is required", field)
		}
	}
	for field, v := range map[string]string{
		"siteName": s.SiteName, "locale": s.Locale, "title": s.Title, "description": s.Description,
		"tagline": s.Tagline, "hero.headline": s.Hero.Headline, "hero.subhead": s.Hero.Subhead,
		"features.heading": s.Features.Heading, "steps.heading": s.Steps.Heading,
		"pricing.heading": s.Pricing.Heading, "pricing.text": s.Pricing.Text, "pricing.price": s.Pricing.Price,
		"pricing.currency": s.Pricing.Currency, "faq.heading": s.FAQ.Heading,
		"stores.comingSoon": s.Stores.ComingSoon, "stores.appStoreLabel": s.Stores.AppStoreLabel,
		"stores.playStoreLabel": s.Stores.PlayStoreLabel, "app.operatingSystem": s.App.OperatingSystem,
		"app.category": s.App.Category, "publisher.name": s.Publisher.Name, "publisher.email": s.Publisher.Email,
	} {
		req(field, v)
	}
	if n := utf8.RuneCountInString(s.Title); n > maxTitle {
		bad("title is %d characters, max %d", n, maxTitle)
	}
	if n := utf8.RuneCountInString(s.Description); n > maxDescription {
		bad("description is %d characters, max %d", n, maxDescription)
	}
	if u, err := url.Parse(s.URL); err != nil || u.Scheme != "https" || u.Host == "" || !strings.HasSuffix(s.URL, "/") {
		bad("url must be an absolute https URL ending in /, got %q", s.URL)
	}
	for _, u := range []struct{ f, v string }{
		{"stores.appStoreUrl", s.Stores.AppStoreURL}, {"stores.playStoreUrl", s.Stores.PlayStoreURL},
		{"publisher.url", s.Publisher.URL},
	} {
		if p, err := url.Parse(u.v); err != nil || p.Scheme != "https" || p.Host == "" {
			bad("%s must be an https URL, got %q", u.f, u.v)
		}
	}
	if !dateRe.MatchString(s.LastModified) {
		bad("lastModified must be YYYY-MM-DD, got %q", s.LastModified)
	}
	for name, c := range map[string]string{
		"background": s.Theme.Background, "text": s.Theme.Text, "muted": s.Theme.Muted, "primary": s.Theme.Primary,
		"accent": s.Theme.Accent, "hero": s.Theme.Hero, "heroLabel": s.Theme.HeroLabel, "card": s.Theme.Card, "line": s.Theme.Line,
	} {
		if !hexColour.MatchString(c) {
			bad("theme.%s must be a #RRGGBB colour, got %q", name, c)
		}
	}
	if len(s.Features.Items) == 0 {
		bad("features.items needs at least one item")
	}
	if len(s.Steps.Items) == 0 {
		bad("steps.items needs at least one item")
	}
	if len(s.FAQ.Items) == 0 {
		bad("faq.items needs at least one item")
	}
	for i, it := range append(append([]Item{}, s.Features.Items...), s.Steps.Items...) {
		if it.Title == "" || it.Text == "" {
			bad("feature/step item %d needs a title and text", i)
		}
	}
	for i, q := range s.FAQ.Items {
		if q.Question == "" || q.Answer == "" {
			bad("faq.items[%d] needs a question and an answer", i)
		}
	}
	for i, l := range s.Footer.Links {
		if l.Label == "" || l.Href == "" {
			bad("footer.links[%d] needs a label and an href", i)
		}
	}
	images := append([]Image{s.OGImage, s.Brand.Logo}, s.Features.Screenshots...)
	if s.Hero.Image.Src != "" {
		images = append(images, s.Hero.Image)
	}
	assets := []string{s.Icons.SVG, s.Icons.PNG32, s.Icons.AppleTouch}
	for _, f := range []Font{s.Fonts.Heading, s.Fonts.Body} {
		if f.Family == "" {
			continue // system font stack
		}
		if len(f.Faces) == 0 {
			bad("fonts.%q needs at least one face", f.Family)
		}
		for _, face := range f.Faces {
			assets = append(assets, face.File)
		}
	}
	for _, im := range images {
		assets = append(assets, im.Src)
		if im.Src == "" || im.Width <= 0 || im.Height <= 0 {
			bad("image %q needs src, width and height", im.Src)
		}
		if im.Alt == "" && im.Src != s.Brand.Logo.Src { // only the decorative logo may have empty alt
			bad("image %q needs alt text", im.Src)
		}
	}
	if fsys != nil {
		for _, a := range assets {
			if !strings.HasPrefix(a, "/static/") {
				bad("asset %q must start with /static/", a)
				continue
			}
			if _, err := fs.Stat(fsys, strings.TrimPrefix(a, "/")); err != nil {
				bad("asset %q not found in embedded static files", a)
			}
		}
	}
	return errors.Join(errs...)
}

var current atomic.Pointer[Site]

// Set installs the loaded site; Current returns it (nil before Set).
func Set(s *Site)    { current.Store(s) }
func Current() *Site { return current.Load() }
