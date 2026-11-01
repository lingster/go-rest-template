import { ApiError } from './api/client.ts';

/** Human readable message for any thrown value. */
export function errorMessage(err: unknown): string {
	if (err instanceof ApiError) return err.message;
	if (err instanceof Error) return err.message;
	return 'Something went wrong. Please try again.';
}

/** Per-field validation errors from a `validation_error` response. */
export function fieldErrors(err: unknown): Record<string, string> {
	return err instanceof ApiError && err.code === 'validation_error' ? (err.details ?? {}) : {};
}

/** Only allow same-origin absolute paths as post-login redirect targets. */
export function safeRedirect(target: string | null | undefined, fallback = '/dashboard'): string {
	if (!target || !target.startsWith('/') || target.startsWith('//') || target.includes('\\')) {
		return fallback;
	}
	return target;
}

export const PASSWORD_MIN = 8;
export const PASSWORD_MAX = 128;

/** Client side password policy check (mirrors the contract). Returns an error or undefined. */
export function passwordPolicyError(password: string): string | undefined {
	if (password.length < PASSWORD_MIN)
		return `Password must be at least ${PASSWORD_MIN} characters.`;
	if (password.length > PASSWORD_MAX) return `Password must be at most ${PASSWORD_MAX} characters.`;
	return undefined;
}

export function formatDate(value: string | null | undefined): string {
	if (!value) return 'Never';
	const d = new Date(value);
	return Number.isNaN(d.getTime()) ? value : d.toLocaleString();
}
