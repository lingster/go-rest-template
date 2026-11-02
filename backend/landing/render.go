package landing

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
)

type obj = map[string]any

// AbsoluteURL resolves a site-relative path (starting with /) against the canonical URL.
func (s *Site) AbsoluteURL(path string) string {
	return strings.TrimSuffix(s.URL, "/") + path
}

// OGImageURL is the absolute Open Graph image URL.
func (s *Site) OGImageURL() string { return s.AbsoluteURL(s.OGImage.Src) }

// Page is the data handed to the Beego template.
type Page struct {
	*Site
	StoreLinksLive bool
	AppJSONLD      template.JS
	FAQJSONLD      template.JS
	ThemeCSSPath   string
}

// ThemeCSSPath is where the generated theme stylesheet (colours and @font-face) is served.
const ThemeCSSPath = "/static/landing/theme.css"

// NewPage builds the template data. storeLinksLive comes from deployment config, not content.
func (s *Site) NewPage(storeLinksLive bool) Page {
	return Page{Site: s, StoreLinksLive: storeLinksLive, AppJSONLD: jsonLD(s.AppSchema()), FAQJSONLD: jsonLD(s.FAQSchema()),
		ThemeCSSPath: ThemeCSSPath}
}

// jsonLD encodes v for an application/ld+json block (json escapes <, > and &, so it cannot close the tag).
func jsonLD(v any) template.JS {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return template.JS(b)
}

// AppSchema is the schema.org MobileApplication description.
func (s *Site) AppSchema() obj {
	return obj{
		"@context": "https://schema.org", "@type": "MobileApplication",
		"name": s.SiteName, "description": s.Description, "url": s.URL, "image": s.OGImageURL(),
		"operatingSystem": s.App.OperatingSystem, "applicationCategory": s.App.Category,
		"offers":    obj{"@type": "Offer", "price": s.Pricing.Price, "priceCurrency": s.Pricing.Currency},
		"publisher": obj{"@type": "Organization", "name": s.Publisher.Name, "url": s.Publisher.URL, "email": s.Publisher.Email},
	}
}

// FAQSchema is the schema.org FAQPage matching the visible FAQ.
func (s *Site) FAQSchema() obj {
	qs := make([]obj, 0, len(s.FAQ.Items))
	for _, f := range s.FAQ.Items {
		qs = append(qs, obj{"@type": "Question", "name": f.Question, "acceptedAnswer": obj{"@type": "Answer", "text": f.Answer}})
	}
	return obj{"@context": "https://schema.org", "@type": "FAQPage", "mainEntity": qs}
}

// RobotsTxt renders robots.txt.
func (s *Site) RobotsTxt() string {
	var b strings.Builder
	b.WriteString("User-agent: *\nAllow: /\n")
	for _, d := range s.Robots.Disallow {
		fmt.Fprintf(&b, "Disallow: %s\n", d)
	}
	fmt.Fprintf(&b, "\nSitemap: %s\n", s.AbsoluteURL("/sitemap.xml"))
	return b.String()
}

// SitemapXML renders sitemap.xml (the landing page only).
func (s *Site) SitemapXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n" +
		fmt.Sprintf("  <url><loc>%s</loc><lastmod>%s</lastmod></url>\n</urlset>\n", s.URL, s.LastModified)
}

const (
	systemBodyFont    = `system-ui,-apple-system,"Segoe UI",Roboto,"Helvetica Neue",Arial,sans-serif`
	systemHeadingFont = systemBodyFont
)

// fontStack is the CSS font-family list for f: the bundled family first when set, then the fallback.
func fontStack(f Font, fallback string) string {
	if f.Family == "" {
		return fallback
	}
	return fmt.Sprintf("%q,%s", f.Family, fallback)
}

// ThemeCSS renders the CSS custom properties and @font-face rules from the content file. It is served
// as a stylesheet (not inlined) so the page stays within the strict CSP.
func (s *Site) ThemeCSS() string {
	var b strings.Builder
	for _, f := range []Font{s.Fonts.Heading, s.Fonts.Body} {
		for _, face := range f.Faces {
			fmt.Fprintf(&b, "@font-face{font-family:%q;font-weight:%d;font-style:normal;font-display:swap;src:url(%s) format(\"woff2\")}\n",
				f.Family, face.Weight, face.File)
		}
	}
	t := s.Theme
	fmt.Fprintf(&b, ":root{--bg:%s;--text:%s;--muted:%s;--primary:%s;--accent:%s;--hero:%s;--hero-label:%s;--card:%s;--line:%s;"+
		"--font-heading:%s;--font-body:%s}\n",
		t.Background, t.Text, t.Muted, t.Primary, t.Accent, t.Hero, t.HeroLabel, t.Card, t.Line,
		fontStack(s.Fonts.Heading, systemHeadingFont), fontStack(s.Fonts.Body, systemBodyFont))
	return b.String()
}
