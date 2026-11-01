<script lang="ts">
	import { resolve } from '$app/paths';
	import * as authApi from '#lib/api/auth.ts';
	import Alert from '#lib/components/Alert.svelte';
	import AuthShell from '#lib/components/AuthShell.svelte';
	import Button from '#lib/components/Button.svelte';
	import FormField from '#lib/components/FormField.svelte';
	import { errorMessage } from '#lib/forms.ts';

	let email = $state('');
	let loading = $state(false);
	let error = $state('');
	let done = $state('');

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		loading = true;
		error = '';
		try {
			done = (await authApi.forgotPassword(email.trim())).message;
		} catch (err) {
			error = errorMessage(err);
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head><title>Forgot password</title></svelte:head>

<AuthShell
	title="Reset your password"
	subtitle="We will email you a link to choose a new password."
>
	{#if done}
		<Alert variant="success">{done}</Alert>
	{:else}
		<form class="space-y-4" onsubmit={submit}>
			{#if error}
				<Alert variant="error">{error}</Alert>
			{/if}
			<FormField
				id="email"
				label="Email"
				type="email"
				autocomplete="email"
				required
				bind:value={email}
			/>
			<Button type="submit" fullWidth {loading}>Send reset link</Button>
		</form>
	{/if}

	{#snippet footer()}
		<a href={resolve('/(auth)/login')}>Back to sign in</a>
	{/snippet}
</AuthShell>
