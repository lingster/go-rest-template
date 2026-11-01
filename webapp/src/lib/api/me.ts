import { api } from './http.ts';
import type {
	MessageResponse,
	Passkey,
	PasskeyCeremony,
	RecoveryCodes,
	TotpSetup,
	User
} from './types.ts';

export const getMe = () => api.request<User>('/me');

export const updateMe = (body: { name: string }) =>
	api.request<User>('/me', { method: 'PATCH', body });

export const changePassword = (body: { current_password: string; new_password: string }) =>
	api.request<MessageResponse>('/me/change-password', { method: 'POST', body });

export const totpSetup = () => api.request<TotpSetup>('/me/2fa/totp/setup', { method: 'POST' });

export const totpEnable = (code: string) =>
	api.request<RecoveryCodes>('/me/2fa/totp/enable', { method: 'POST', body: { code } });

export const totpDisable = (body: { password: string; code: string }) =>
	api.request<MessageResponse>('/me/2fa/totp/disable', { method: 'POST', body });

export const regenerateRecoveryCodes = (code: string) =>
	api.request<RecoveryCodes>('/me/2fa/recovery-codes', { method: 'POST', body: { code } });

export const listPasskeys = () => api.request<Passkey[]>('/me/passkeys');

export const passkeyRegisterBegin = () =>
	api.request<PasskeyCeremony>('/me/passkeys/register/begin', { method: 'POST' });

export const passkeyRegisterFinish = (body: {
	session_id: string;
	name: string;
	credential: unknown;
}) => api.request<Passkey>('/me/passkeys/register/finish', { method: 'POST', body });

export const deletePasskey = (id: string) =>
	api.request<void>(`/me/passkeys/${encodeURIComponent(id)}`, { method: 'DELETE' });
