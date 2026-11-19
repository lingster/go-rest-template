import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AuthResponse, User } from '../api/types.ts';

vi.mock('../api/auth.ts', () => ({ refresh: vi.fn(), logout: vi.fn() }));
vi.mock('../api/me.ts', () => ({ getMe: vi.fn() }));

const user: User = {
	id: 'u1',
	email: 'a@b.com',
	name: 'Ada',
	role: 'user',
	email_verified: true,
	is_active: true,
	totp_enabled: false,
	passkey_count: 0,
	has_password: true,
	identities: [],
	created_at: '',
	updated_at: ''
};
const tokens = { access_token: 'a', token_type: 'Bearer', expires_in: 900 };
const authResponse: AuthResponse = { mfa_required: false, tokens, user };

function memoryStorage() {
	const m = new Map<string, string>();
	return {
		getItem: (k: string) => m.get(k) ?? null,
		setItem: (k: string, v: string) => void m.set(k, v),
		removeItem: (k: string) => void m.delete(k),
		get length() {
			return m.size;
		}
	};
}

async function load() {
	vi.resetModules();
	const authApi = await import('../api/auth.ts');
	const meApi = await import('../api/me.ts');
	const mod = await import('./session.svelte.ts');
	const { api } = await import('../api/http.ts');
	return { authApi, meApi, api, ...mod };
}

describe('session store', () => {
	beforeEach(() => {
		vi.resetAllMocks();
		vi.unstubAllGlobals();
		vi.stubGlobal('localStorage', memoryStorage());
	});

	it('signIn keeps the access token in memory and persists nothing', async () => {
		const { session, api } = await load();
		const fetchMock = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
		vi.stubGlobal('fetch', fetchMock);
		expect(session.isAuthenticated).toBe(false);
		session.signIn(authResponse);
		expect(session.user).toEqual(user);
		expect(session.isAuthenticated).toBe(true);
		expect(session.isAdmin).toBe(false);
		expect(localStorage.length).toBe(0);
		await api.request('/me');
		expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe('Bearer a');
	});

	it('isAdmin reflects role', async () => {
		const { session } = await load();
		session.signIn({ ...authResponse, user: { ...user, role: 'admin' } });
		expect(session.isAdmin).toBe(true);
	});

	it('init restores the session from the refresh cookie', async () => {
		const { session, authApi, meApi } = await load();
		vi.mocked(authApi.refresh).mockResolvedValue(tokens);
		vi.mocked(meApi.getMe).mockResolvedValue(user);
		await Promise.all([session.init(), session.init()]);
		expect(authApi.refresh).toHaveBeenCalledTimes(1);
		expect(authApi.refresh).toHaveBeenCalledWith(undefined);
		expect(session.ready).toBe(true);
		expect(session.user).toEqual(user);
	});

	it('init treats a rejected cookie refresh as signed out', async () => {
		const { session, authApi, meApi } = await load();
		vi.mocked(authApi.refresh).mockRejectedValue(new Error('no session'));
		await session.init();
		expect(session.ready).toBe(true);
		expect(session.user).toBeNull();
		expect(meApi.getMe).not.toHaveBeenCalled();
	});

	it('init migrates a legacy localStorage refresh token once, then deletes it', async () => {
		const { session, authApi, meApi, LEGACY_REFRESH_TOKEN_KEY } = await load();
		localStorage.setItem(LEGACY_REFRESH_TOKEN_KEY, 'r1');
		vi.mocked(authApi.refresh).mockResolvedValue(tokens);
		vi.mocked(meApi.getMe).mockResolvedValue(user);
		await session.init();
		expect(authApi.refresh).toHaveBeenCalledWith('r1');
		expect(session.user).toEqual(user);
		expect(localStorage.getItem(LEGACY_REFRESH_TOKEN_KEY)).toBeNull();
	});

	it('init deletes the legacy token even when it is rejected', async () => {
		const { session, authApi, LEGACY_REFRESH_TOKEN_KEY } = await load();
		localStorage.setItem(LEGACY_REFRESH_TOKEN_KEY, 'bad');
		vi.mocked(authApi.refresh).mockRejectedValue(new Error('nope'));
		await session.init();
		expect(session.user).toBeNull();
		expect(localStorage.getItem(LEGACY_REFRESH_TOKEN_KEY)).toBeNull();
	});

	it('a sign-in that finishes during the restore is kept', async () => {
		const { session, authApi } = await load();
		let reject!: (e: Error) => void;
		vi.mocked(authApi.refresh).mockReturnValue(new Promise((_, r) => (reject = r)));
		const init = session.init();
		session.signIn(authResponse);
		reject(new Error('no session'));
		await init;
		expect(session.user).toEqual(user);
	});

	it('a restore that resolves after a newer sign-in does not overwrite it', async () => {
		const { session, authApi, meApi } = await load();
		const stale: User = { ...user, id: 'u0', email: 'old@b.com' };
		vi.mocked(authApi.refresh).mockResolvedValue(tokens);
		let resolveMe!: (u: User) => void;
		vi.mocked(meApi.getMe).mockReturnValue(new Promise((r) => (resolveMe = r)));
		const init = session.init();
		await vi.waitFor(() => expect(meApi.getMe).toHaveBeenCalled());
		session.signIn(authResponse);
		resolveMe(stale);
		await init;
		expect(session.user).toEqual(user);
	});

	it('signOut drops tokens delivered while logout was queued', async () => {
		const { session, authApi, api } = await load();
		const fetchMock = vi.fn().mockResolvedValue(new Response('{}', { status: 200 }));
		vi.stubGlobal('fetch', fetchMock);
		session.signIn(authResponse);
		vi.mocked(authApi.logout).mockImplementation(async () => {
			session.signIn({ ...authResponse, tokens: { ...tokens, access_token: 'late' } });
		});
		await session.signOut();
		expect(session.user).toBeNull();
		await api.request('/me');
		expect(fetchMock.mock.calls[0][1].headers.Authorization).toBeUndefined();
	});

	it('signOut clears locally and revokes the cookie session', async () => {
		const { session, authApi } = await load();
		vi.mocked(authApi.logout).mockResolvedValue(undefined);
		session.signIn(authResponse);
		await session.signOut();
		expect(session.user).toBeNull();
		expect(authApi.logout).toHaveBeenCalledWith();
	});
});
