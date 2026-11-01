import { createApiClient } from './client.ts';

export const DEFAULT_API_URL = 'http://localhost:8080';

/**
 * `PUBLIC_API_URL` is inlined at build time (see `envPrefix` in vite.config.ts), so it
 * must be set when running `vite build` / `npm run deploy`.
 */
export const API_URL: string = import.meta.env.PUBLIC_API_URL || DEFAULT_API_URL;

/** Shared application API client instance. */
export const api = createApiClient({ baseUrl: API_URL });
