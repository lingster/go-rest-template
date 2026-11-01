<script lang="ts">
	import { onMount } from 'svelte';
	import * as authApi from '../api/auth.ts';
	import type { LoginResponse } from '../api/types.ts';
	import { errorMessage } from '../forms.ts';
	import { buttonOptions, credentialFrom, loadGoogleIdentity } from '../google.ts';

	interface Props {
		/** Called with the server's response (session or 2FA challenge). */
		onLogin: (res: LoginResponse) => void | Promise<void>;
		onError: (message: string) => void;
	}

	let { onLogin, onError }: Props = $props();

	let container = $state<HTMLDivElement>();
	let visible = $state(false);

	async function handleCredential(response: { credential?: string }) {
		const idToken = credentialFrom(response);
		if (!idToken) return;
		try {
			await onLogin(await authApi.googleLogin(idToken));
		} catch (err) {
			onError(errorMessage(err));
		}
	}

	onMount(() => {
		void (async () => {
			try {
				const { google } = await authApi.providers();
				if (!google.enabled || !google.web_client_id || !container) return;
				const gsi = await loadGoogleIdentity();
				gsi.initialize({
					client_id: google.web_client_id,
					callback: handleCredential,
					ux_mode: 'popup',
					auto_select: false
				});
				gsi.renderButton(container, buttonOptions(container.clientWidth));
				visible = true;
			} catch {
				// Google sign-in is optional: when disabled, offline or blocked, show nothing.
			}
		})();
	});
</script>

<div class:hidden={!visible} class="space-y-4">
	<div class="relative text-center text-xs text-slate-500">or</div>
	<div bind:this={container} class="flex justify-center" data-testid="google-signin"></div>
</div>
