import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import type { LoginResponse } from './api/types.ts';
import { gotoPath } from './navigation.ts';
import { mfaChallenge } from './stores/mfa.svelte.ts';
import { session } from './stores/session.svelte.ts';

/**
 * Finish a first-factor login (password or Google): go to the 2FA step when the server asked
 * for one, otherwise start the session and continue to `redirectTo`.
 */
export async function finishLogin(res: LoginResponse, redirectTo: string): Promise<void> {
	if (res.mfa_required) {
		mfaChallenge.set(res.mfa_token, res.methods, redirectTo);
		await goto(resolve('/(auth)/login/2fa'));
		return;
	}
	session.signIn(res);
	await gotoPath(redirectTo);
}
