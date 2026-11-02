package controllers

import (
	"github.com/beego/beego/v2/server/web"

	"github.com/lingster/go-rest-template/backend/container"
	"github.com/lingster/go-rest-template/backend/landing"
)

const (
	landingPageCache = "public, max-age=300"
	landingTextCache = "public, max-age=3600"
)

// LandingController serves the public marketing page, robots.txt, sitemap.xml and the theme stylesheet,
// all rendered from the loaded landing.Site (conf/landing.json). No auth, no JavaScript.
type LandingController struct{ web.Controller }

// site returns the loaded content, or aborts with the JSON 404 when none is loaded.
func (c *LandingController) site() *landing.Site {
	s := landing.Current()
	if s == nil {
		c.Abort("404")
	}
	return s
}

// Get renders the landing page.
func (c *LandingController) Get() {
	s := c.site()
	c.Data["Page"] = s.NewPage(container.Get().Cfg.LandingStoreLinksLive)
	c.TplName = "landing/index.tpl"
	c.Ctx.Output.Header("Cache-Control", landingPageCache)
	if err := c.Render(); err != nil {
		c.Abort("500")
	}
}

// Robots serves robots.txt.
func (c *LandingController) Robots() { c.text("text/plain; charset=utf-8", c.site().RobotsTxt()) }

// Sitemap serves sitemap.xml.
func (c *LandingController) Sitemap() {
	c.text("application/xml; charset=utf-8", c.site().SitemapXML())
}

// ThemeCSS serves the generated colours and @font-face rules.
func (c *LandingController) ThemeCSS() { c.text("text/css; charset=utf-8", c.site().ThemeCSS()) }

func (c *LandingController) text(contentType, body string) {
	h := c.Ctx.Output
	h.Header("Content-Type", contentType)
	h.Header("Cache-Control", landingTextCache)
	_ = h.Body([]byte(body))
}
