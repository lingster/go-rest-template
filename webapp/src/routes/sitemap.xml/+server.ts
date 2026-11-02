import { buildSitemap } from '#lib/seo.ts';
import type { RequestHandler } from './$types';

export const prerender = true;

export const GET: RequestHandler = () =>
	new Response(buildSitemap(new Date().toISOString().slice(0, 10)), {
		headers: { 'Content-Type': 'application/xml; charset=utf-8' }
	});
