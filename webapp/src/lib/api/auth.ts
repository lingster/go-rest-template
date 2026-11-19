import { api } from './http.ts';
import type {
	AuthResponse,
	LoginResponse,
	MessageResponse,
	PasskeyCeremony,
	Providers,
	RegisterResponse,
	Tokens
} from './types.ts';

const post = <T>(path: string, body?: unknown) =>
	api.request<T>(path, { method: 'POST', body, auth: false });

export const register = (body: { email: string; password: string; name: string }) =>
	post<RegisterResponse>('/auth/register', body);

export const login = (body: { email: string; password: string }) =>
	post<LoginResponse>('/auth/login', body);

export const login2fa = (body: { mfa_token: string; code: string }) =>
	post<AuthResponse>('/auth/login/2fa', body);

/**
 * Uses the HttpOnly refresh cookie. `legacyRefreshToken` is only sent once, to move a session stored
 * by an older build into the cookie.
 */
export const refresh = (legacyRefreshToken?: string) =>
	post<Tokens>(
		'/auth/refresh',
		legacyRefreshToken ? { refresh_token: legacyRefreshToken } : undefined
	);

/** Revokes the refresh cookie's session and clears the cookie. */
export const logout = () => post<void>('/auth/logout');

export const verifyEmail = (token: string) =>
	post<MessageResponse>('/auth/verify-email', { token });

export const resendVerification = (email: string) =>
	post<MessageResponse>('/auth/resend-verification', { email });

export const forgotPassword = (email: string) =>
	post<MessageResponse>('/auth/forgot-password', { email });

export const resetPassword = (body: { token: string; password: string }) =>
	post<MessageResponse>('/auth/reset-password', body);

export const passkeyLoginBegin = (email?: string) =>
	post<PasskeyCeremony>('/auth/passkey/login/begin', email ? { email } : {});

export const passkeyLoginFinish = (body: { session_id: string; credential: unknown }) =>
	post<AuthResponse>('/auth/passkey/login/finish', body);

export const providers = () =>
	api.request<Providers>('/auth/providers', { method: 'GET', auth: false });

export const googleLogin = (id_token: string) => post<LoginResponse>('/auth/google', { id_token });
