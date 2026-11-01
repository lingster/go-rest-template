import tailwindcss from '@tailwindcss/vite';
import { loadEnv } from 'vite';
import { defineConfig } from 'vitest/config';
import adapter from '@sveltejs/adapter-cloudflare';
import { sveltekit } from '@sveltejs/kit/vite';

const DEFAULT_API_URL = 'http://localhost:8080';

/** Origin of the REST API, used to allow-list `connect-src` in the CSP. */
function apiOrigin(mode: string): `${string}://${string}.${string}` {
	const env = { ...loadEnv(mode, process.cwd(), 'PUBLIC_'), ...process.env };
	try {
		return new URL(env.PUBLIC_API_URL || DEFAULT_API_URL).origin as ReturnType<typeof apiOrigin>;
	} catch {
		return DEFAULT_API_URL as ReturnType<typeof apiOrigin>;
	}
}

export default defineConfig(({ mode }) => ({
	// Expose PUBLIC_* vars to import.meta.env (inlined at build time).
	envPrefix: ['VITE_', 'PUBLIC_'],
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			adapter: adapter(),
			csp: {
				mode: 'auto',
				directives: {
					'default-src': ['self'],
					'script-src': ['self'],
					'style-src': ['self'],
					'connect-src': ['self', apiOrigin(mode)],
					'img-src': ['self', 'data:'],
					'frame-ancestors': ['none'],
					'object-src': ['none'],
					'base-uri': ['self'],
					'form-action': ['self']
				}
			}
		})
	],
	test: {
		expect: { requireAssertions: true },
		projects: [
			{
				extends: './vite.config.ts',
				test: {
					name: 'server',
					environment: 'node',
					include: ['src/**/*.{test,spec}.{js,ts}'],
					exclude: ['src/**/*.svelte.{test,spec}.{js,ts}']
				}
			}
		]
	}
}));
