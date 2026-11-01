<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import * as adminApi from '#lib/api/admin.ts';
	import type { User } from '#lib/api/types.ts';
	import UserActions from '#lib/components/admin/UserActions.svelte';
	import Alert from '#lib/components/Alert.svelte';
	import Card from '#lib/components/Card.svelte';
	import { errorMessage, formatDate } from '#lib/forms.ts';

	let user = $state<User | null>(null);
	let error = $state('');
	let notice = $state('');

	onMount(async () => {
		try {
			user = await adminApi.getUser(page.params.id ?? '');
		} catch (err) {
			error = errorMessage(err);
		}
	});

	const rows = $derived(
		user
			? [
					['ID', user.id],
					['Email', user.email],
					['Role', user.role],
					['Active', user.is_active ? 'Yes' : 'No'],
					['Email verified', user.email_verified ? 'Yes' : 'No'],
					['2FA enabled', user.totp_enabled ? 'Yes' : 'No'],
					['Passkeys', String(user.passkey_count)],
					['Created', formatDate(user.created_at)],
					['Updated', formatDate(user.updated_at)]
				]
			: []
	);
</script>

<svelte:head><title>User</title></svelte:head>

<p class="text-sm"><a href={resolve('/(admin)/admin/users')}>&larr; All users</a></p>
<h1 class="mt-2 text-2xl font-bold">{user?.name ?? 'User'}</h1>

<div class="mt-6 space-y-4">
	{#if error}<Alert variant="error">{error}</Alert>{/if}
	{#if notice}<Alert variant="success">{notice}</Alert>{/if}

	{#if user}
		<Card title="Details">
			<dl class="grid gap-x-6 gap-y-3 text-sm sm:grid-cols-2">
				{#each rows as [label, value] (label)}
					<div>
						<dt class="text-slate-500 dark:text-slate-400">{label}</dt>
						<dd class="font-medium break-all">{value}</dd>
					</div>
				{/each}
			</dl>
		</Card>
		<Card title="Actions">
			<UserActions
				{user}
				onupdate={(u) => (user = u)}
				ondelete={() => goto(resolve('/(admin)/admin/users'))}
				onerror={(m) => (error = m)}
				onnotice={(m) => (notice = m)}
			/>
		</Card>
		<p class="text-sm">
			<a href="{resolve('/(admin)/admin/audit-logs')}?user_id={encodeURIComponent(user.id)}">
				View audit logs for this user
			</a>
		</p>
	{/if}
</div>
