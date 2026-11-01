import { api } from './http.ts';
import type {
	AuthResponse,
	LoginResponse,
	MessageResponse,
	PasskeyCeremony,
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

export const refresh = (refresh_token: string) => post<Tokens>('/auth/refresh', { refresh_token });

export const logout = (refresh_token: string) => post<void>('/auth/logout', { refresh_token });

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
