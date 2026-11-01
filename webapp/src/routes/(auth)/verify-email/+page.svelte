<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import * as authApi from '#lib/api/auth.ts';
	import Alert from '#lib/components/Alert.svelte';
	import AuthShell from '#lib/components/AuthShell.svelte';
	import { errorMessage } from '#lib/forms.ts';

	type Status = 'loading' | 'success' | 'error';
	let status = $state<Status>('loading');
	let message = $state('');

	onMount(async () => {
		const token = page.url.searchParams.get('token');
		if (!token) {
			status = 'error';
			message = 'This verification link is missing its token.';
			return;
		}
		try {
			message = (await authApi.verifyEmail(token)).message || 'Your email has been verified.';
			status = 'success';
		} catch (err) {
			message = errorMessage(err);
			status = 'error';
		}
	});
</script>

<svelte:head><title>Verify email</title></svelte:head>

<AuthShell title="Email verification">
	{#if status === 'loading'}
		<p class="text-center text-sm" role="status">Verifying your email...</p>
	{:else if status === 'success'}
		<Alert variant="success">{message}</Alert>
		<a href={resolve('/(auth)/login')} class="block text-center text-sm">Continue to sign in</a>
	{:else}
		<Alert variant="error">{message}</Alert>
		<a href={resolve('/(auth)/login')} class="block text-center text-sm">
			Back to sign in (you can request a new verification email there)
		</a>
	{/if}
</AuthShell>
