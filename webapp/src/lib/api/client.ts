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

/** Callbacks through which the client talks to whoever owns the session (DIP). */
export interface AuthHooks {
	getAccessToken(): string | null;
	getRefreshToken(): string | null;
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
		const headers: Record<string, string> = { Accept: 'application/json' };
		if (opts.body !== undefined) headers['Content-Type'] = 'application/json';
		const token = withAuth ? hooks?.getAccessToken() : null;
		if (token) headers.Authorization = `Bearer ${token}`;
		try {
			return await doFetch(buildUrl(path, opts.query), {
				method: opts.method ?? 'GET',
				headers,
				body: opts.body === undefined ? undefined : JSON.stringify(opts.body)
			});
		} catch {
			throw new ApiError(0, 'network_error', 'Unable to reach the server. Please try again.');
		}
	}

	async function doRefresh(): Promise<void> {
		const refreshToken = hooks?.getRefreshToken();
		if (!hooks || !refreshToken) {
			hooks?.onAuthFailure();
			throw new ApiError(401, 'unauthorized', 'Session expired');
		}
		const res = await send(
			'/auth/refresh',
			{ method: 'POST', body: { refresh_token: refreshToken } },
			false
		);
		if (!res.ok) {
			const err = await toApiError(res);
			hooks.onAuthFailure();
			throw err;
		}
		hooks.onTokens((await res.json()) as Tokens);
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
