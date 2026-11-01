import type { MfaMethod } from '../api/types.ts';

/** Pending MFA challenge between /login and /login/2fa. Memory only, never persisted. */
class MfaChallengeStore {
	token = $state<string | null>(null);
	methods = $state<MfaMethod[]>([]);
	redirect = $state<string>('/dashboard');

	set(token: string, methods: MfaMethod[], redirect: string): void {
		this.token = token;
		this.methods = methods;
		this.redirect = redirect;
	}

	clear(): void {
		this.token = null;
		this.methods = [];
	}
}

export const mfaChallenge = new MfaChallengeStore();
