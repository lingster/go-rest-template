<script lang="ts">
	import { resolve } from '$app/paths';
	import * as authApi from '#lib/api/auth.ts';
	import Alert from '#lib/components/Alert.svelte';
	import AuthShell from '#lib/components/AuthShell.svelte';
	import Button from '#lib/components/Button.svelte';
	import GoogleSignInButton from '#lib/components/GoogleSignInButton.svelte';
	import FormField from '#lib/components/FormField.svelte';
	import PasswordField from '#lib/components/PasswordField.svelte';
	import { errorMessage, fieldErrors, passwordPolicyError } from '#lib/forms.ts';
	import { finishLogin } from '#lib/login-flow.ts';

	let name = $state('');
	let email = $state('');
	let password = $state('');
	let loading = $state(false);
	let error = $state('');
	let errors = $state<Record<string, string>>({});
	let done = $state('');

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		errors = {};
		const policy = passwordPolicyError(password);
		if (policy) {
			errors = { password: policy };
			return;
		}
		loading = true;
		try {
			const res = await authApi.register({ name: name.trim(), email: email.trim(), password });
			done = res.message || 'Account created. Check your email to verify your address.';
		} catch (err) {
			error = errorMessage(err);
			errors = fieldErrors(err);
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head><title>Create account</title></svelte:head>

<AuthShell title="Create your account">
	{#if done}
		<Alert variant="success">{done}</Alert>
		<a href={resolve('/(auth)/login')} class="block text-center text-sm">Go to sign in</a>
	{:else}
		<form class="space-y-4" onsubmit={submit} novalidate>
			{#if error}
				<Alert variant="error">{error}</Alert>
			{/if}
			<FormField
				id="name"
				label="Name"
				autocomplete="name"
				required
				bind:value={name}
				error={errors.name}
			/>
			<FormField
				id="email"
				label="Email"
				type="email"
				autocomplete="email"
				required
				bind:value={email}
				error={errors.email}
			/>
			<PasswordField
				id="password"
				label="Password"
				autocomplete="new-password"
				required
				hint="8 to 128 characters."
				bind:value={password}
				error={errors.password}
			/>
			<Button type="submit" fullWidth {loading}>Create account</Button>
		</form>
		<GoogleSignInButton
			onLogin={(res) => finishLogin(res, '/dashboard')}
			onError={(m) => (error = m)}
		/>
	{/if}

	{#snippet footer()}
		Already have an account? <a href={resolve('/(auth)/login')}>Sign in</a>
	{/snippet}
</AuthShell>
