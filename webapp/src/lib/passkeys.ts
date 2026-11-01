import { startAuthentication, startRegistration } from '@simplewebauthn/browser';
import * as authApi from './api/auth.ts';
import * as meApi from './api/me.ts';
import type { AuthResponse, Passkey } from './api/types.ts';

type CreationOptions = Parameters<typeof startRegistration>[0]['optionsJSON'];
type RequestOptionsJSON = Parameters<typeof startAuthentication>[0]['optionsJSON'];

/** Run the full passkey login ceremony. Omit email for a discoverable credential. */
export async function loginWithPasskey(email?: string): Promise<AuthResponse> {
	const { session_id, options } = await authApi.passkeyLoginBegin(email);
	const credential = await startAuthentication({
		optionsJSON: (options as { publicKey: unknown }).publicKey as RequestOptionsJSON
	});
	return authApi.passkeyLoginFinish({ session_id, credential });
}

/** Register a new passkey for the signed-in user. */
export async function registerPasskey(name: string): Promise<Passkey> {
	const { session_id, options } = await meApi.passkeyRegisterBegin();
	const credential = await startRegistration({
		optionsJSON: (options as { publicKey: unknown }).publicKey as CreationOptions
	});
	return meApi.passkeyRegisterFinish({ session_id, name, credential });
}

/** True when the user cancelled / dismissed the browser's WebAuthn prompt. */
export function isPasskeyCancelled(err: unknown): boolean {
	return err instanceof Error && (err.name === 'NotAllowedError' || err.name === 'AbortError');
}
