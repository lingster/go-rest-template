<script lang="ts">
	import { resolve } from '$app/paths';
	import Alert from '#lib/components/Alert.svelte';
	import Card from '#lib/components/Card.svelte';
	import { session } from '#lib/stores/session.svelte.ts';
</script>

<svelte:head><title>Dashboard</title></svelte:head>

<h1 class="text-2xl font-bold">Welcome, {session.user?.name}</h1>
<p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{session.user?.email}</p>

<div class="mt-6 space-y-4">
	{#if session.user && !session.user.totp_enabled && session.user.passkey_count === 0}
		<Alert variant="warning">
			Your account is protected by a password only.
			<a href={resolve('/(app)/settings/security')}
				>Enable two-factor authentication or add a passkey.</a
			>
		</Alert>
	{/if}

	<div class="grid gap-4 sm:grid-cols-2">
		<Card title="Profile" description="Update your name and change your password.">
			<a href={resolve('/(app)/settings/profile')} class="text-sm">Manage profile</a>
		</Card>
		<Card title="Security" description="Two-factor authentication and passkeys.">
			<p class="mb-2 text-sm">
				2FA: <strong>{session.user?.totp_enabled ? 'enabled' : 'off'}</strong>, passkeys:
				<strong>{session.user?.passkey_count ?? 0}</strong>
			</p>
			<a href={resolve('/(app)/settings/security')} class="text-sm">Manage security</a>
		</Card>
	</div>
</div>
