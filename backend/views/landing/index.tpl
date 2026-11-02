{{with .Page}}<!doctype html>
<html lang="{{.Locale}}">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <meta name="description" content="{{.Description}}">
  <link rel="canonical" href="{{.URL}}">
  <meta name="robots" content="index, follow, max-image-preview:large">
  <meta name="theme-color" content="{{.Theme.Hero}}">
  <link rel="icon" href="{{.Icons.SVG}}" type="image/svg+xml">
  <link rel="icon" href="{{.Icons.PNG32}}" sizes="32x32" type="image/png">
  <link rel="apple-touch-icon" href="{{.Icons.AppleTouch}}">
  <meta property="og:type" content="website">
  <meta property="og:site_name" content="{{.SiteName}}">
  <meta property="og:locale" content="{{.Locale}}">
  <meta property="og:title" content="{{.Title}}">
  <meta property="og:description" content="{{.Description}}">
  <meta property="og:url" content="{{.URL}}">
  <meta property="og:image" content="{{.OGImageURL}}">
  <meta property="og:image:width" content="{{.OGImage.Width}}">
  <meta property="og:image:height" content="{{.OGImage.Height}}">
  <meta property="og:image:alt" content="{{.OGImage.Alt}}">
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:title" content="{{.Title}}">
  <meta name="twitter:description" content="{{.Description}}">
  <meta name="twitter:image" content="{{.OGImageURL}}">
  {{with .Fonts.Heading.Preload}}<link rel="preload" href="{{.}}" as="font" type="font/woff2" crossorigin>{{end}}
  {{with .Fonts.Body.Preload}}<link rel="preload" href="{{.}}" as="font" type="font/woff2" crossorigin>{{end}}
  <link rel="stylesheet" href="{{.ThemeCSSPath}}">
  <link rel="stylesheet" href="/static/landing/landing.css">
  <script type="application/ld+json">{{.AppJSONLD}}</script>
  <script type="application/ld+json">{{.FAQJSONLD}}</script>
</head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="hero">
  <div class="wrap">
    <div class="brand"><img src="{{.Brand.Logo.Src}}" alt="{{.Brand.Logo.Alt}}" width="{{.Brand.Logo.Width}}" height="{{.Brand.Logo.Height}}"><span>{{.SiteName}}</span></div>
    <div class="hero-grid{{if .Hero.Image.Src}} has-shot{{end}}">
      <div>
        {{with .Hero.Eyebrow}}<p class="label">{{.}}</p>{{end}}
        <h1>{{.Hero.Headline}}</h1>
        <p class="lead">{{.Hero.Subhead}}</p>
        {{template "store" .}}
        {{with .Hero.Note}}<p class="fine">{{.}}</p>{{end}}
      </div>
      {{with .Hero.Image}}{{if .Src}}<div class="hero-shot">
        <img src="{{.Src}}" width="{{.Width}}" height="{{.Height}}" alt="{{.Alt}}" fetchpriority="high">
      </div>{{end}}{{end}}
    </div>
  </div>
</header>
<main id="main">
  <section aria-labelledby="features">
    <div class="wrap">
      {{with .Features.Eyebrow}}<p class="label">{{.}}</p>{{end}}
      <h2 id="features">{{.Features.Heading}}</h2>
      {{with .Features.Intro}}<p class="intro">{{.}}</p>{{end}}
      <ul class="grid">
        {{range .Features.Items}}<li class="card"><h3>{{with .Icon}}<span aria-hidden="true">{{.}}</span> {{end}}{{.Title}}</h3><p>{{.Text}}</p></li>
        {{end}}
      </ul>
      {{if .Features.Screenshots}}<div class="shots">
        {{range .Features.Screenshots}}<figure><img loading="lazy" src="{{.Src}}" width="{{.Width}}" height="{{.Height}}" alt="{{.Alt}}">{{with .Caption}}<figcaption>{{.}}</figcaption>{{end}}</figure>
        {{end}}
      </div>{{end}}
    </div>
  </section>
  <section class="alt" aria-labelledby="how">
    <div class="wrap">
      {{with .Steps.Eyebrow}}<p class="label">{{.}}</p>{{end}}
      <h2 id="how">{{.Steps.Heading}}</h2>
      <ol class="grid steps">
        {{range .Steps.Items}}<li class="card"><h3>{{.Title}}</h3><p>{{.Text}}</p></li>
        {{end}}
      </ol>
    </div>
  </section>
  <section aria-labelledby="pricing">
    <div class="wrap">
      {{with .Pricing.Eyebrow}}<p class="label">{{.}}</p>{{end}}
      <h2 id="pricing">{{.Pricing.Heading}}</h2>
      <div class="card price"><p>{{.Pricing.Text}}</p></div>
    </div>
  </section>
  <section class="alt" aria-labelledby="faq">
    <div class="wrap">
      {{with .FAQ.Eyebrow}}<p class="label">{{.}}</p>{{end}}
      <h2 id="faq">{{.FAQ.Heading}}</h2>
      {{range .FAQ.Items}}<details><summary>{{.Question}}</summary><p>{{.Answer}}</p></details>
      {{end}}
    </div>
  </section>
  <section aria-labelledby="get">
    <div class="wrap">
      <h2 id="get">{{.Stores.Heading}}</h2>
      {{template "store" .}}
    </div>
  </section>
</main>
<footer>
  <div class="wrap">
    <ul>
      {{range .Footer.Links}}<li><a href="{{.Href}}">{{.Label}}</a></li>
      {{end}}
      <li><a href="mailto:{{.Publisher.Email}}">{{.Publisher.Email}}</a></li>
    </ul>
    {{with .Footer.Disclaimer}}<p>{{.}}</p>{{end}}
    <p>&copy; {{.Publisher.Name}}</p>
  </div>
</footer>
</body>
</html>{{end}}
{{define "store"}}{{if .StoreLinksLive}}<div class="cta"><a class="btn" href="{{.Stores.AppStoreURL}}">{{.Stores.AppStoreLabel}}</a><a class="btn" href="{{.Stores.PlayStoreURL}}">{{.Stores.PlayStoreLabel}}</a></div>{{else}}<p class="soon">{{.Stores.ComingSoon}}</p>{{end}}{{end}}
