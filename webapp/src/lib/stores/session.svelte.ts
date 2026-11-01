import { api } from '../api/http.ts';
import * as authApi from '../api/auth.ts';
import * as meApi from '../api/me.ts';
import type { AuthResponse, Tokens, User } from '../api/types.ts';

export const REFRESH_TOKEN_KEY = 'saas.refresh_token';

function readStored(): string | null {
	try {
		return globalThis.localStorage?.getItem(REFRESH_TOKEN_KEY) ?? null;
	} catch {
		return null;
	}
}

function writeStored(value: string | null): void {
	try {
		if (value) globalThis.localStorage?.setItem(REFRESH_TOKEN_KEY, value);
		else globalThis.localStorage?.removeItem(REFRESH_TOKEN_KEY);
	} catch {
		// storage unavailable (private mode etc.): session lives in memory only
	}
}

/**
 * Auth state. The access token is held in memory only; the refresh token is
 * mirrored to localStorage so a page reload can restore the session.
 */
export class SessionStore {
	user = $state<User | null>(null);
	/** True once the initial restore attempt has finished. */
	ready = $state(false);

	#accessToken: string | null = null;
	#refreshToken: string | null = null;
	#initPromise: Promise<void> | null = null;

	constructor() {
		api.setAuthHooks({
			getAccessToken: () => this.#accessToken,
			getRefreshToken: () => this.#refreshToken ?? readStored(),
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
		this.#refreshToken = t.refresh_token;
		writeStored(t.refresh_token);
	}

	/** Restore the session from a stored refresh token (idempotent). */
	init(): Promise<void> {
		this.#initPromise ??= this.#restore();
		return this.#initPromise;
	}

	async #restore(): Promise<void> {
		try {
			const stored = readStored();
			if (stored) {
				this.#setTokens(await authApi.refresh(stored));
				this.user = await meApi.getMe();
			}
		} catch {
			this.clear();
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

	/** Revoke the refresh token server-side (best effort) and clear local state. */
	async signOut(): Promise<void> {
		const token = this.#refreshToken ?? readStored();
		this.clear();
		if (token) {
			try {
				await authApi.logout(token);
			} catch {
				// already signed out locally
			}
		}
	}

	/** Drop all local auth state. */
	clear(): void {
		this.#accessToken = null;
		this.#refreshToken = null;
		this.user = null;
		writeStored(null);
	}
}

export const session = new SessionStore();
