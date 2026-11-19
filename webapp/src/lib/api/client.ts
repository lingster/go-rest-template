import type { ApiErrorBody, Tokens } from './types.ts';

export class ApiError extends Error {
	readonly status: number;
	readonly code: string;
	readonly details?: Record<string, string>;

	constructor(status: number, code: string, message: string, details?: Record<string, string>) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.code = code;
		this.details = details;
	}

	is(code: string): boolean {
		return this.code === code;
	}
}

/**
 * Opts into the cookie session transport: the API keeps the refresh token in an HttpOnly cookie
 * (scoped to /api/v1/auth) instead of returning it in JSON, so page scripts never see it.
 */
export const SESSION_TRANSPORT_HEADER = 'X-Session-Transport';

/** Callbacks through which the client talks to whoever owns the session (DIP). */
export interface AuthHooks {
	getAccessToken(): string | null;
	onTokens(tokens: Tokens): void;
	onAuthFailure(): void;
}

export interface RequestOptions {
	method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE';
	body?: unknown;
	query?: Record<string, string | number | boolean | undefined | null>;
	/** Attach the bearer token and refresh+retry on 401. Defaults to true. */
	auth?: boolean;
}

export interface ApiClientOptions {
	baseUrl: string;
	fetch?: typeof fetch;
}

/**
 * Endpoints whose responses set or clear the refresh cookie. They are sent one at a time so a
 * response can never overwrite the cookie written by a later request (e.g. a slow session restore
 * clobbering a fresh login, or an in-flight refresh re-creating the cookie after logout).
 */
const COOKIE_WRITING_PATHS = new Set([
	'/auth/login',
	'/auth/login/2fa',
	'/auth/google',
	'/auth/passkey/login/finish',
	'/auth/refresh',
	'/auth/logout'
]);

/** 401 codes that mean "your access token is no good" (as opposed to bad credentials / MFA code). */
const REFRESHABLE_CODES = new Set(['unauthorized', 'token_expired']);

async function toApiError(res: Response): Promise<ApiError> {
	let body: Partial<ApiErrorBody> | undefined;
	try {
		body = (await res.json()) as Partial<ApiErrorBody>;
	} catch {
		// non-JSON error body
	}
	const err = body?.error;
	return new ApiError(
		res.status,
		err?.code ?? 'unknown_error',
		err?.message ?? (res.statusText || 'Request failed'),
		err?.details
	);
}

export function createApiClient({ baseUrl, fetch: fetchImpl }: ApiClientOptions) {
	const root = `${baseUrl.replace(/\/+$/, '')}/api/v1`;
	let hooks: AuthHooks | null = null;
	let refreshing: Promise<void> | null = null;
	/** Tail of the cookie-writing request queue (see COOKIE_WRITING_PATHS). */
	let cookieQueue: Promise<unknown> = Promise.resolve();

	/** Runs task after every previously queued cookie-writing request has settled. */
	function serialized<T>(task: () => Promise<T>): Promise<T> {
		const run = cookieQueue.then(task, task);
		cookieQueue = run.catch(() => undefined);
		return run;
	}

	const doFetch: typeof fetch = (...args) => (fetchImpl ?? globalThis.fetch)(...args);

	function buildUrl(path: string, query?: RequestOptions['query']): string {
		const qs = new URLSearchParams();
		for (const [k, v] of Object.entries(query ?? {})) {
			if (v !== undefined && v !== null && v !== '') qs.set(k, String(v));
		}
		const s = qs.toString();
		return `${root}${path}${s ? `?${s}` : ''}`;
	}

	async function send(path: string, opts: RequestOptions, withAuth: boolean): Promise<Response> {
		const headers: Record<string, string> = {
			Accept: 'application/json',
			[SESSION_TRANSPORT_HEADER]: 'cookie'
		};
		if (opts.body !== undefined) headers['Content-Type'] = 'application/json';
		const token = withAuth ? hooks?.getAccessToken() : null;
		if (token) headers.Authorization = `Bearer ${token}`;
		const fetchIt = () =>
			doFetch(buildUrl(path, opts.query), {
				method: opts.method ?? 'GET',
				headers,
				credentials: 'include',
				body: opts.body === undefined ? undefined : JSON.stringify(opts.body)
			});
		try {
			// The cookie is stored when the response headers arrive, i.e. when fetch resolves.
			return await (COOKIE_WRITING_PATHS.has(path) ? serialized(fetchIt) : fetchIt());
		} catch {
			throw new ApiError(0, 'network_error', 'Unable to reach the server. Please try again.');
		}
	}

	/** The refresh token travels in the HttpOnly cookie, so the request has no body. */
	async function doRefresh(): Promise<void> {
		const res = await send('/auth/refresh', { method: 'POST' }, false);
		if (!res.ok) {
			const err = await toApiError(res);
			hooks?.onAuthFailure();
			throw err;
		}
		hooks?.onTokens((await res.json()) as Tokens);
	}

	/** Single-flight: concurrent 401s share one refresh call (tokens rotate, so double refresh would revoke the family). */
	function refreshOnce(): Promise<void> {
		refreshing ??= doRefresh().finally(() => {
			refreshing = null;
		});
		return refreshing;
	}

	async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
		const withAuth = opts.auth ?? true;
		let res = await send(path, opts, withAuth);

		if (res.status === 401 && withAuth && hooks) {
			const err = await toApiError(res.clone());
			if (REFRESHABLE_CODES.has(err.code) || err.code === 'unknown_error') {
				await refreshOnce();
				res = await send(path, opts, withAuth);
			}
		}

		if (!res.ok) throw await toApiError(res);
		if (res.status === 204) return undefined as T;
		const text = await res.text();
		return (text ? JSON.parse(text) : undefined) as T;
	}

	return {
		request,
		setAuthHooks(h: AuthHooks | null) {
			hooks = h;
		}
	};
}

export type ApiClient = ReturnType<typeof createApiClient>;
