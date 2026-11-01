<script lang="ts">
	import QRCode from 'qrcode';
	import * as meApi from '../../api/me.ts';
	import type { TotpSetup } from '../../api/types.ts';
	import { errorMessage } from '../../forms.ts';
	import { session } from '../../stores/session.svelte.ts';
	import Alert from '../Alert.svelte';
	import Button from '../Button.svelte';
	import Card from '../Card.svelte';
	import FormField from '../FormField.svelte';
	import PasswordField from '../PasswordField.svelte';
	import RecoveryCodes from './RecoveryCodes.svelte';

	type Mode = 'idle' | 'setup' | 'disable' | 'regenerate';

	let mode = $state<Mode>('idle');
	let setup = $state<TotpSetup | null>(null);
	let qr = $state('');
	let code = $state('');
	let password = $state('');
	let recoveryCodes = $state<string[]>([]);
	let loading = $state(false);
	let error = $state('');
	let notice = $state('');

	const enabled = $derived(session.user?.totp_enabled ?? false);
	// Google-created accounts have no password: the authentication code alone confirms disabling.
	const hasPassword = $derived(session.user?.has_password ?? true);

	function reset(next: Mode = 'idle') {
		mode = next;
		code = '';
		password = '';
		error = '';
		setup = null;
		qr = '';
	}

	/** Run an async action with shared loading/error handling. */
	async function run(action: () => Promise<void>) {
		loading = true;
		error = '';
		notice = '';
		try {
			await action();
		} catch (err) {
			error = errorMessage(err);
		} finally {
			loading = false;
		}
	}

	const startSetup = () =>
		run(async () => {
			const res = await meApi.totpSetup();
			qr = await QRCode.toDataURL(res.otpauth_url, { margin: 1, width: 192 });
			setup = res;
			mode = 'setup';
		});

	const confirmSetup = (e: SubmitEvent) => {
		e.preventDefault();
		return run(async () => {
			recoveryCodes = (await meApi.totpEnable(code.trim())).recovery_codes;
			await session.refreshUser();
			reset();
		});
	};

	const disable = (e: SubmitEvent) => {
		e.preventDefault();
		return run(async () => {
			await meApi.totpDisable({
				password: hasPassword ? password : undefined,
				code: code.trim()
			});
			await session.refreshUser();
			reset();
			notice = 'Two-factor authentication disabled.';
		});
	};

	const regenerate = (e: SubmitEvent) => {
		e.preventDefault();
		return run(async () => {
			recoveryCodes = (await meApi.regenerateRecoveryCodes(code.trim())).recovery_codes;
			reset();
		});
	};
</script>

<Card
	title="Two-factor authentication"
	description="Protect your account with a time-based one-time code from an authenticator app."
>
	<div class="space-y-4">
		{#if error}<Alert variant="error">{error}</Alert>{/if}
		{#if notice}<Alert variant="success">{notice}</Alert>{/if}

		{#if recoveryCodes.length}
			<RecoveryCodes codes={recoveryCodes} ondismiss={() => (recoveryCodes = [])} />
		{:else if mode === 'setup' && setup}
			<form class="space-y-4" onsubmit={confirmSetup}>
				<p class="text-sm">
					Scan this QR code with your authenticator app, then enter the 6-digit code to confirm.
				</p>
				<img src={qr} alt="TOTP QR code" width="192" height="192" class="rounded-md bg-white p-1" />
				<p class="text-sm break-all">
					Or enter this key manually: <code class="font-mono">{setup.secret}</code>
				</p>
				<FormField
					id="totp-code"
					label="Authentication code"
					inputmode="numeric"
					autocomplete="one-time-code"
					maxlength={6}
					required
					bind:value={code}
				/>
				<div class="flex gap-2">
					<Button type="submit" {loading}>Confirm and enable</Button>
					<Button variant="ghost" onclick={() => reset()}>Cancel</Button>
				</div>
			</form>
		{:else if mode === 'disable'}
			<form class="space-y-4" onsubmit={disable}>
				{#if hasPassword}
					<PasswordField
						id="dis-password"
						label="Password"
						autocomplete="current-password"
						required
						bind:value={password}
					/>
				{/if}
				<FormField
					id="dis-code"
					label="Authentication code"
					inputmode="numeric"
					autocomplete="one-time-code"
					required
					bind:value={code}
				/>
				<div class="flex gap-2">
					<Button type="submit" variant="danger" {loading}>Disable 2FA</Button>
					<Button variant="ghost" onclick={() => reset()}>Cancel</Button>
				</div>
			</form>
		{:else if mode === 'regenerate'}
			<form class="space-y-4" onsubmit={regenerate}>
				<FormField
					id="regen-code"
					label="Authentication code"
					hint="Generating new codes invalidates the old ones."
					inputmode="numeric"
					autocomplete="one-time-code"
					required
					bind:value={code}
				/>
				<div class="flex gap-2">
					<Button type="submit" {loading}>Generate new codes</Button>
					<Button variant="ghost" onclick={() => reset()}>Cancel</Button>
				</div>
			</form>
		{:else if enabled}
			<p class="text-sm">Status: <strong>enabled</strong></p>
			<div class="flex flex-wrap gap-2">
				<Button variant="secondary" onclick={() => reset('regenerate')}>
					Regenerate recovery codes
				</Button>
				<Button variant="danger" onclick={() => reset('disable')}>Disable</Button>
			</div>
		{:else}
			<p class="text-sm">Status: <strong>not enabled</strong></p>
			<Button {loading} onclick={startSetup}>Set up authenticator app</Button>
		{/if}
	</div>
</Card>
