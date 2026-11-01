/** Content-Security-Policy allowances for Google Identity Services. Kept free of `import.meta.env` so vite.config.ts can import it. */

/** CSP origins GIS needs (https://developers.google.com/identity/gsi/web/guides/get-google-api-clientid#content_security_policy). */
export const GSI_CSP = {
	'script-src': ['https://accounts.google.com/gsi/client'],
	'frame-src': ['https://accounts.google.com/gsi/'],
	'connect-src': ['https://accounts.google.com/gsi/'],
	'style-src': ['https://accounts.google.com/gsi/style']
} as const;

/** Extend CSP directives with the GIS allowances when Google sign-in is enabled; otherwise return them unchanged. */
export function withGoogleCsp(
	directives: Record<string, string[]>,
	enabled: boolean
): Record<string, string[]> {
	if (!enabled) return directives;
	const out = { ...directives };
	for (const [name, sources] of Object.entries(GSI_CSP)) {
		out[name] = [...(out[name] ?? []), ...sources];
	}
	return out;
}
