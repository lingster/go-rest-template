import { api } from '../api/http.ts';
import * as authApi from '../api/auth.ts';
import * as meApi from '../api/me.ts';
import type { AuthResponse, Tokens, User } from '../api/types.ts';

/** Where builds before the cookie transport kept the refresh token; only read to migrate. */
export const LEGACY_REFRESH_TOKEN_KEY = 'saas.refresh_token';

/** Return and delete a refresh token left in localStorage by an older build. */
function takeLegacyRefreshToken(): string | undefined {
	try {
		const token = globalThis.localStorage?.getItem(LEGACY_REFRESH_TOKEN_KEY);
		if (token) globalThis.localStorage?.removeItem(LEGACY_REFRESH_TOKEN_KEY);
		return token || undefined;
	} catch {
		return undefined; // storage unavailable (private mode etc.)
	}
}

/**
 * Auth state. The access token is held in memory only; the refresh token lives in an HttpOnly
 * cookie set by the API, so scripts (including injected ones) cannot read it, and a page reload
 * restores the session by calling `/auth/refresh`.
 */
export class SessionStore {
	user = $state<User | null>(null);
	/** True once the initial restore attempt has finished. */
	ready = $state(false);

	#accessToken: string | null = null;
	#initPromise: Promise<void> | null = null;

	constructor() {
		api.setAuthHooks({
			getAccessToken: () => this.#accessToken,
			onTokens: (t) => this.#setTokens(t),
			onAuthFailure: () => this.clear()
		});
	}

	get isAuthenticated(): boolean {
		return this.user !== null;
	}

	get isAdmin(): boolean {
		return this.user?.role === 'admin';
	}

	#setTokens(t: Tokens): void {
		this.#accessToken = t.access_token;
	}

	/** Restore the session from the refresh cookie (idempotent). */
	init(): Promise<void> {
		this.#initPromise ??= this.#restore();
		return this.#initPromise;
	}

	async #restore(): Promise<void> {
		try {
			const tokens = await authApi.refresh(takeLegacyRefreshToken());
			if (this.user) return; // signed in while the restore was in flight: keep that session
			this.#setTokens(tokens);
			this.user = await meApi.getMe();
		} catch {
			// no (valid) cookie: signed out, unless a sign-in finished meanwhile
			if (!this.user) this.clear();
		} finally {
			this.ready = true;
		}
	}

	signIn(res: AuthResponse): void {
		this.#setTokens(res.tokens);
		this.user = res.user;
		this.ready = true;
	}

	setUser(user: User): void {
		this.user = user;
	}

	async refreshUser(): Promise<User> {
		const user = await meApi.getMe();
		this.user = user;
		return user;
	}

	/** Revoke the session server-side and clear its cookie (best effort), then clear local state. */
	async signOut(): Promise<void> {
		this.clear();
		try {
			await authApi.logout();
		} catch {
			// already signed out locally
		}
	}

	/** Drop all local auth state. */
	clear(): void {
		this.#accessToken = null;
		this.user = null;
	}
}

export const session = new SessionStore();
