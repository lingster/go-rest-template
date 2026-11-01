<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { gotoPath } from '#lib/navigation.ts';
	import * as authApi from '#lib/api/auth.ts';
	import Alert from '#lib/components/Alert.svelte';
	import AuthShell from '#lib/components/AuthShell.svelte';
	import Button from '#lib/components/Button.svelte';
	import FormField from '#lib/components/FormField.svelte';
	import { errorMessage } from '#lib/forms.ts';
	import { mfaChallenge } from '#lib/stores/mfa.svelte.ts';
	import { session } from '#lib/stores/session.svelte.ts';

	let code = $state('');
	let useRecovery = $state(false);
	let loading = $state(false);
	let error = $state('');

	onMount(() => {
		// A reload loses the in-memory challenge; start again.
		if (!mfaChallenge.token) void goto(resolve('/(auth)/login'), { replaceState: true });
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!mfaChallenge.token) return;
		loading = true;
		error = '';
		try {
			const res = await authApi.login2fa({ mfa_token: mfaChallenge.token, code: code.trim() });
			const target = mfaChallenge.redirect;
			mfaChallenge.clear();
			session.signIn(res);
			await gotoPath(target);
		} catch (err) {
			error = errorMessage(err);
		} finally {
			loading = false;
		}
	}

	function toggle() {
		useRecovery = !useRecovery;
		code = '';
		error = '';
	}
</script>

<svelte:head><title>Two-factor authentication</title></svelte:head>

<AuthShell
	title="Two-factor authentication"
	subtitle={useRecovery
		? 'Enter one of your recovery codes.'
		: 'Enter the 6-digit code from your authenticator app.'}
>
	<form class="space-y-4" onsubmit={submit}>
		{#if error}
			<Alert variant="error">{error}</Alert>
		{/if}
		{#if useRecovery}
			<FormField
				id="code"
				label="Recovery code"
				autocomplete="one-time-code"
				autocapitalize="none"
				required
				bind:value={code}
			/>
		{:else}
			<FormField
				id="code"
				label="Authentication code"
				inputmode="numeric"
				autocomplete="one-time-code"
				maxlength={6}
				pattern="[0-9]*"
				required
				bind:value={code}
			/>
		{/if}
		<Button type="submit" fullWidth {loading}>Verify</Button>
	</form>
	<div class="text-center text-sm">
		<button
			type="button"
			class="text-indigo-600 hover:underline dark:text-indigo-400"
			onclick={toggle}
		>
			{useRecovery ? 'Use authenticator app instead' : 'Use a recovery code'}
		</button>
	</div>

	{#snippet footer()}
		<a href={resolve('/(auth)/login')}>Back to sign in</a>
	{/snippet}
</AuthShell>
