<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import * as authApi from '#lib/api/auth.ts';
	import Alert from '#lib/components/Alert.svelte';
	import AuthShell from '#lib/components/AuthShell.svelte';
	import Button from '#lib/components/Button.svelte';
	import PasswordField from '#lib/components/PasswordField.svelte';
	import { errorMessage, fieldErrors, passwordPolicyError } from '#lib/forms.ts';

	const token = $derived(page.url.searchParams.get('token') ?? '');

	let password = $state('');
	let confirm = $state('');
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
		if (password !== confirm) {
			errors = { confirm: 'Passwords do not match.' };
			return;
		}
		loading = true;
		try {
			done = (await authApi.resetPassword({ token, password })).message;
		} catch (err) {
			error = errorMessage(err);
			errors = fieldErrors(err);
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head><title>Reset password</title></svelte:head>

<AuthShell title="Choose a new password">
	{#if !token}
		<Alert variant="error">This reset link is missing its token. Request a new one.</Alert>
		<a href={resolve('/(auth)/forgot-password')} class="block text-center text-sm"
			>Request new link</a
		>
	{:else if done}
		<Alert variant="success">{done || 'Password updated.'}</Alert>
		<a href={resolve('/(auth)/login')} class="block text-center text-sm">Sign in</a>
	{:else}
		<form class="space-y-4" onsubmit={submit} novalidate>
			{#if error}
				<Alert variant="error">{error}</Alert>
			{/if}
			<PasswordField
				id="password"
				label="New password"
				autocomplete="new-password"
				required
				hint="8 to 128 characters."
				bind:value={password}
				error={errors.password}
			/>
			<PasswordField
				id="confirm"
				label="Confirm new password"
				autocomplete="new-password"
				required
				bind:value={confirm}
				error={errors.confirm}
			/>
			<Button type="submit" fullWidth {loading}>Update password</Button>
		</form>
	{/if}
</AuthShell>
