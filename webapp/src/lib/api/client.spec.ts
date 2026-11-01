import { describe, expect, it, vi } from 'vitest';
import { ApiError, createApiClient, type AuthHooks } from './client.ts';
import type { Tokens } from './types.ts';

const json = (status: number, body?: unknown) =>
	new Response(body === undefined ? null : JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});

const errBody = (code: string, message = 'msg', details?: Record<string, string>) => ({
	error: { code, message, details }
});

const newTokens: Tokens = {
	access_token: 'new-access',
	refresh_token: 'new-refresh',
	token_type: 'Bearer',
	expires_in: 900
};

function setup(fetchImpl: ReturnType<typeof vi.fn>) {
	const state = { access: 'old-access', refresh: 'old-refresh' as string | null };
	const hooks: AuthHooks = {
		getAccessToken: () => state.access,
		getRefreshToken: () => state.refresh,
		onTokens: vi.fn((t: Tokens) => {
			state.access = t.access_token;
			state.refresh = t.refresh_token;
		}),
		onAuthFailure: vi.fn()
	};
	const client = createApiClient({
		baseUrl: 'http://api.test/',
		fetch: fetchImpl as unknown as typeof fetch
	});
	client.setAuthHooks(hooks);
	return { client, hooks, state };
}

describe('api client', () => {
	it('parses the contract error body into ApiError', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValue(json(422, errBody('validation_error', 'bad', { email: 'invalid' })));
		const { client } = setup(fetchMock);
		const err = await client
			.request<ApiError>('/auth/register', { method: 'POST', auth: false })
			.catch((e: unknown) => e as ApiError);
		expect(err).toBeInstanceOf(ApiError);
		expect(err.status).toBe(422);
		expect(err.code).toBe('validation_error');
		expect(err.message).toBe('bad');
		expect(err.details).toEqual({ email: 'invalid' });
	});

	it('falls back to unknown_error for non-JSON bodies', async () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response('boom', { status: 502 }));
		const { client } = setup(fetchMock);
		const err = await client.request<ApiError>('/x').catch((e: unknown) => e as ApiError);
		expect(err).toBeInstanceOf(ApiError);
		expect(err.status).toBe(502);
		expect(err.code).toBe('unknown_error');
	});

	it('wraps network failures as ApiError status 0', async () => {
		const { client } = setup(vi.fn().mockRejectedValue(new TypeError('fail')));
		const err = await client.request<ApiError>('/x').catch((e: unknown) => e as ApiError);
		expect(err.code).toBe('network_error');
		expect(err.status).toBe(0);
	});

	it('builds URLs, attaches bearer, sends JSON and handles 204', async () => {
		const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
		const { client } = setup(fetchMock);
		const out = await client.request<ApiError>('/admin/users', {
			method: 'POST',
			body: { a: 1 },
			query: { page: 2, q: '', per_page: 20 }
		});
		expect(out).toBeUndefined();
		const [url, init] = fetchMock.mock.calls[0];
		expect(url).toBe('http://api.test/api/v1/admin/users?page=2&per_page=20');
		expect(init.headers.Authorization).toBe('Bearer old-access');
		expect(init.headers['Content-Type']).toBe('application/json');
		expect(init.body).toBe('{"a":1}');
	});

	it('does not attach bearer when auth is false', async () => {
		const fetchMock = vi.fn().mockResolvedValue(json(200, {}));
		const { client } = setup(fetchMock);
		await client.request<ApiError>('/auth/login', { method: 'POST', body: {}, auth: false });
		expect(fetchMock.mock.calls[0][1].headers.Authorization).toBeUndefined();
	});

	it('refreshes on 401 unauthorized and retries with the new token', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(json(401, errBody('unauthorized')))
			.mockResolvedValueOnce(json(200, newTokens))
			.mockResolvedValueOnce(json(200, { id: 'u1' }));
		const { client, hooks } = setup(fetchMock);
		const res = await client.request<{ id: string }>('/me');
		expect(res.id).toBe('u1');
		expect(fetchMock).toHaveBeenCalledTimes(3);
		expect(fetchMock.mock.calls[1][0]).toBe('http://api.test/api/v1/auth/refresh');
		expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toEqual({ refresh_token: 'old-refresh' });
		expect(fetchMock.mock.calls[2][1].headers.Authorization).toBe('Bearer new-access');
		expect(hooks.onTokens).toHaveBeenCalledWith(newTokens);
		expect(hooks.onAuthFailure).not.toHaveBeenCalled();
	});

	it('shares a single refresh between concurrent 401s', async () => {
		let refreshCalls = 0;
		const fetchMock = vi.fn(async (url: string, init: RequestInit) => {
			if (url.endsWith('/auth/refresh')) {
				refreshCalls++;
				await new Promise((r) => setTimeout(r, 10));
				return json(200, newTokens);
			}
			const auth = (init.headers as Record<string, string>).Authorization;
			return auth === 'Bearer new-access'
				? json(200, { ok: true })
				: json(401, errBody('token_expired'));
		});
		const { client } = setup(fetchMock);
		const results = await Promise.all([
			client.request<ApiError>('/me'),
			client.request<ApiError>('/me/passkeys'),
			client.request<ApiError>('/admin/stats')
		]);
		expect(results).toHaveLength(3);
		expect(refreshCalls).toBe(1);
	});

	it('logs out when refresh fails', async () => {
		const fetchMock = vi
			.fn()
			.mockResolvedValueOnce(json(401, errBody('unauthorized')))
			.mockResolvedValueOnce(json(401, errBody('invalid_token', 'revoked')));
		const { client, hooks } = setup(fetchMock);
		const err = await client.request<ApiError>('/me').catch((e: unknown) => e as ApiError);
		expect(err).toBeInstanceOf(ApiError);
		expect(err.code).toBe('invalid_token');
		expect(hooks.onAuthFailure).toHaveBeenCalledTimes(1);
	});

	it('logs out when there is no refresh token', async () => {
		const fetchMock = vi.fn().mockResolvedValueOnce(json(401, errBody('unauthorized')));
		const { client, hooks, state } = setup(fetchMock);
		state.refresh = null;
		await expect(client.request<ApiError>('/me')).rejects.toBeInstanceOf(ApiError);
		expect(hooks.onAuthFailure).toHaveBeenCalledTimes(1);
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

	it('does not refresh on invalid_credentials and does not loop after a retry', async () => {
		const fetchMock = vi.fn().mockResolvedValueOnce(json(401, errBody('invalid_credentials')));
		const { client } = setup(fetchMock);
		const err = await client
			.request<ApiError>('/me/change-password', { method: 'POST' })
			.catch((e: unknown) => e as ApiError);
		expect(err.code).toBe('invalid_credentials');
		expect(fetchMock).toHaveBeenCalledTimes(1);

		const looping = vi
			.fn()
			.mockResolvedValueOnce(json(401, errBody('unauthorized')))
			.mockResolvedValueOnce(json(200, newTokens))
			.mockResolvedValueOnce(json(401, errBody('unauthorized')));
		const second = setup(looping);
		await expect(second.client.request<ApiError>('/me')).rejects.toMatchObject({ status: 401 });
		expect(looping).toHaveBeenCalledTimes(3);
	});
});
