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
	created_at: '',
	updated_at: ''
};
const tokens = { access_token: 'a', refresh_token: 'r2', token_type: 'Bearer', expires_in: 900 };
const authResponse: AuthResponse = { mfa_required: false, tokens, user };

function memoryStorage() {
	const m = new Map<string, string>();
	return {
		getItem: (k: string) => m.get(k) ?? null,
		setItem: (k: string, v: string) => void m.set(k, v),
		removeItem: (k: string) => void m.delete(k)
	};
}

async function load() {
	vi.resetModules();
	const authApi = await import('../api/auth.ts');
	const meApi = await import('../api/me.ts');
	const mod = await import('./session.svelte.ts');
	return { authApi, meApi, ...mod };
}

describe('session store', () => {
	beforeEach(() => {
		vi.stubGlobal('localStorage', memoryStorage());
	});

	it('signIn stores user and persists only the refresh token', async () => {
		const { session, REFRESH_TOKEN_KEY } = await load();
		expect(session.isAuthenticated).toBe(false);
		session.signIn(authResponse);
		expect(session.user).toEqual(user);
		expect(session.isAuthenticated).toBe(true);
		expect(session.isAdmin).toBe(false);
		expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBe('r2');
	});

	it('isAdmin reflects role', async () => {
		const { session } = await load();
		session.signIn({ ...authResponse, user: { ...user, role: 'admin' } });
		expect(session.isAdmin).toBe(true);
	});

	it('init without stored token just becomes ready', async () => {
		const { session, authApi } = await load();
		await session.init();
		expect(session.ready).toBe(true);
		expect(session.user).toBeNull();
		expect(authApi.refresh).not.toHaveBeenCalled();
	});

	it('init restores session from stored refresh token', async () => {
		const { session, authApi, meApi, REFRESH_TOKEN_KEY } = await load();
		localStorage.setItem(REFRESH_TOKEN_KEY, 'r1');
		vi.mocked(authApi.refresh).mockResolvedValue(tokens);
		vi.mocked(meApi.getMe).mockResolvedValue(user);
		await Promise.all([session.init(), session.init()]);
		expect(authApi.refresh).toHaveBeenCalledTimes(1);
		expect(authApi.refresh).toHaveBeenCalledWith('r1');
		expect(session.user).toEqual(user);
		expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBe('r2');
	});

	it('init clears state when restore fails', async () => {
		const { session, authApi, REFRESH_TOKEN_KEY } = await load();
		localStorage.setItem(REFRESH_TOKEN_KEY, 'bad');
		vi.mocked(authApi.refresh).mockRejectedValue(new Error('nope'));
		await session.init();
		expect(session.ready).toBe(true);
		expect(session.user).toBeNull();
		expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull();
	});

	it('signOut clears locally and revokes the refresh token', async () => {
		const { session, authApi, REFRESH_TOKEN_KEY } = await load();
		vi.mocked(authApi.logout).mockResolvedValue(undefined);
		session.signIn(authResponse);
		await session.signOut();
		expect(session.user).toBeNull();
		expect(localStorage.getItem(REFRESH_TOKEN_KEY)).toBeNull();
		expect(authApi.logout).toHaveBeenCalledWith('r2');
	});
});
