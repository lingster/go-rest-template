/** Shared SEO facts for the web app. scripts/init_project.py rewrites the exported constants. */
export const SITE_NAME = 'SaaS Starter';
export const TAGLINE = "Your app's one-line pitch";
/** Where the web app is served; used for canonical URLs and the sitemap. */
export const WEB_ORIGIN = 'https://app.example.com';
/** The public landing page (served by the API at /); the home page canonicalises to it. */
export const MAIN_SITE_URL = 'https://example.com/';

/** Indexable pages, listed in sitemap.xml. Sign-in and signed-in areas are noindex. */
export const SITEMAP_PATHS = ['/'] as const;

export const canonicalUrl = (path: string): string => `${WEB_ORIGIN}${path === '/' ? '' : path}`;

export function buildSitemap(lastmod: string, paths: readonly string[] = SITEMAP_PATHS): string {
	const urls = paths
		.map((p) => `  <url><loc>${canonicalUrl(p)}</loc><lastmod>${lastmod}</lastmod></url>`)
		.join('\n');
	return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${urls}\n</urlset>\n`;
}
