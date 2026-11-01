/** Types mirroring docs/API_CONTRACT.md. */

export type Role = 'user' | 'admin';

export interface User {
	id: string;
	email: string;
	name: string;
	role: Role;
	email_verified: boolean;
	is_active: boolean;
	totp_enabled: boolean;
	passkey_count: number;
	/** false for accounts created via Google until a password is set through the reset flow */
	has_password: boolean;
	identities: string[];
	created_at: string;
	updated_at: string;
}

export interface Providers {
	google: { enabled: boolean; web_client_id: string };
}

export interface Tokens {
	access_token: string;
	refresh_token: string;
	token_type: string;
	expires_in: number;
}

export interface AuthResponse {
	mfa_required: false;
	tokens: Tokens;
	user: User;
}

export type MfaMethod = 'totp' | 'recovery_code' | 'passkey';

export interface MfaChallenge {
	mfa_required: true;
	mfa_token: string;
	methods: MfaMethod[];
}

export type LoginResponse = AuthResponse | MfaChallenge;

export interface Passkey {
	id: string;
	name: string;
	created_at: string;
	last_used_at: string | null;
}

export interface Paginated<T> {
	items: T[];
	total: number;
	page: number;
	per_page: number;
}

export interface MessageResponse {
	message: string;
}

export interface RegisterResponse extends MessageResponse {
	user: User;
}

export interface TotpSetup {
	secret: string;
	otpauth_url: string;
}

export interface RecoveryCodes {
	recovery_codes: string[];
}

export interface PasskeyCeremony<O = { publicKey: unknown }> {
	session_id: string;
	options: O;
}

export interface AdminStats {
	users_total: number;
	users_active: number;
	users_verified: number;
	users_2fa: number;
}

export interface AdminUserPatch {
	name?: string;
	role?: Role;
	is_active?: boolean;
	email_verified?: boolean;
}

export interface ListParams {
	page?: number;
	per_page?: number;
}

export interface AuditLog {
	id: string;
	user_id: string | null;
	actor_id: string | null;
	action: string;
	ip: string;
	user_agent: string;
	metadata: Record<string, unknown> | null;
	created_at: string;
}

export type ErrorCode =
	| 'validation_error'
	| 'invalid_credentials'
	| 'unauthorized'
	| 'invalid_token'
	| 'token_expired'
	| 'email_not_verified'
	| 'account_disabled'
	| 'forbidden'
	| 'invalid_mfa_code'
	| 'not_found'
	| 'conflict'
	| 'rate_limited'
	| 'internal_error';

export interface ApiErrorBody {
	error: {
		code: string;
		message: string;
		details?: Record<string, string>;
	};
}
