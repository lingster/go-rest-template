<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { gotoPath } from '#lib/navigation.ts';
	import { page } from '$app/state';
	import * as authApi from '#lib/api/auth.ts';
	import { ApiError } from '#lib/api/client.ts';
	import Alert from '#lib/components/Alert.svelte';
	import AuthShell from '#lib/components/AuthShell.svelte';
	import Button from '#lib/components/Button.svelte';
	import GoogleSignInButton from '#lib/components/GoogleSignInButton.svelte';
	import FormField from '#lib/components/FormField.svelte';
	import PasswordField from '#lib/components/PasswordField.svelte';
	import { errorMessage, fieldErrors, safeRedirect } from '#lib/forms.ts';
	import { isPasskeyCancelled, loginWithPasskey } from '#lib/passkeys.ts';
	import { finishLogin } from '#lib/login-flow.ts';
	import { session } from '#lib/stores/session.svelte.ts';

	let email = $state('');
	let password = $state('');
	let loading = $state(false);
	let passkeyLoading = $state(false);
	let error = $state('');
	let errors = $state<Record<string, string>>({});
	let unverified = $state(false);
	let resendMessage = $state('');

	const redirectTo = $derived(safeRedirect(page.url.searchParams.get('redirect')));

	onMount(() => {
		void session.init();
	});

	$effect(() => {
		if (session.ready && session.isAuthenticated) void gotoPath(redirectTo, { replaceState: true });
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		loading = true;
		error = '';
		errors = {};
		unverified = false;
		resendMessage = '';
		try {
			await finishLogin(await authApi.login({ email: email.trim(), password }), redirectTo);
		} catch (err) {
			if (err instanceof ApiError && err.code === 'email_not_verified') unverified = true;
			error = errorMessage(err);
			errors = fieldErrors(err);
		} finally {
			loading = false;
		}
	}

	async function signInWithPasskey() {
		passkeyLoading = true;
		error = '';
		try {
			session.signIn(await loginWithPasskey(email.trim() || undefined));
			await gotoPath(redirectTo);
		} catch (err) {
			if (!isPasskeyCancelled(err)) error = errorMessage(err);
		} finally {
			passkeyLoading = false;
		}
	}

	async function resend() {
		resendMessage = '';
		try {
			resendMessage = (await authApi.resendVerification(email.trim())).message;
		} catch (err) {
			error = errorMessage(err);
		}
	}
</script>

<svelte:head><title>Sign in</title></svelte:head>

<AuthShell title="Sign in to your account">
	<form class="space-y-4" onsubmit={submit} novalidate>
		{#if error}
			<Alert variant="error">{error}</Alert>
		{/if}
		{#if unverified}
			<Alert variant="warning">
				<p>Your email address has not been verified yet.</p>
				<Button variant="secondary" class="mt-2" disabled={!email.trim()} onclick={resend}>
					Resend verification email
				</Button>
			</Alert>
		{/if}
		{#if resendMessage}
			<Alert variant="success">{resendMessage}</Alert>
		{/if}

		<FormField
			id="email"
			label="Email"
			type="email"
			autocomplete="username webauthn"
			required
			bind:value={email}
			error={errors.email}
		/>
		<PasswordField
			id="password"
			label="Password"
			autocomplete="current-password"
			required
			bind:value={password}
			error={errors.password}
		/>
		<div class="text-right text-sm">
			<a href={resolve('/(auth)/forgot-password')}>Forgot your password?</a>
		</div>
		<Button type="submit" fullWidth {loading}>Sign in</Button>
	</form>

	<div class="relative text-center text-xs text-slate-500">or</div>
	<Button variant="secondary" fullWidth loading={passkeyLoading} onclick={signInWithPasskey}>
		Sign in with passkey
	</Button>
	<GoogleSignInButton
		onLogin={(res) => finishLogin(res, redirectTo)}
		onError={(m) => (error = m)}
	/>

	{#snippet footer()}
		No account yet? <a href={resolve('/(auth)/register')}>Create one</a>
	{/snippet}
</AuthShell>
