import { describe, expect, it } from 'vitest';
import { buildSitemap, canonicalUrl, SITEMAP_PATHS, WEB_ORIGIN } from './seo.ts';

describe('seo', () => {
	it('canonical URLs have no trailing slash on the home page', () => {
		expect(canonicalUrl('/')).toBe(WEB_ORIGIN);
		expect(canonicalUrl('/pricing')).toBe(`${WEB_ORIGIN}/pricing`);
	});

	it('sitemap lists every indexable page with a lastmod', () => {
		const xml = buildSitemap('2026-11-02');
		for (const p of SITEMAP_PATHS) expect(xml).toContain(`<loc>${canonicalUrl(p)}</loc>`);
		expect(xml.match(/<lastmod>2026-11-02<\/lastmod>/g)).toHaveLength(SITEMAP_PATHS.length);
	});
});
