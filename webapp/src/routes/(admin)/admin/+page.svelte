<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import * as adminApi from '#lib/api/admin.ts';
	import type { AdminStats } from '#lib/api/types.ts';
	import Alert from '#lib/components/Alert.svelte';
	import Card from '#lib/components/Card.svelte';
	import { errorMessage } from '#lib/forms.ts';

	let stats = $state<AdminStats | null>(null);
	let error = $state('');

	onMount(async () => {
		try {
			stats = await adminApi.getStats();
		} catch (err) {
			error = errorMessage(err);
		}
	});

	const tiles = $derived(
		stats
			? [
					{ label: 'Total users', value: stats.users_total },
					{ label: 'Active users', value: stats.users_active },
					{ label: 'Verified emails', value: stats.users_verified },
					{ label: 'With 2FA', value: stats.users_2fa }
				]
			: []
	);
</script>

<svelte:head><title>Admin</title></svelte:head>

<h1 class="text-2xl font-bold">Admin</h1>

<div class="mt-6 space-y-6">
	{#if error}<Alert variant="error">{error}</Alert>{/if}

	<dl class="grid grid-cols-2 gap-4 lg:grid-cols-4">
		{#each tiles as t (t.label)}
			<div
				class="rounded-xl bg-white p-4 ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
			>
				<dt class="text-sm text-slate-500 dark:text-slate-400">{t.label}</dt>
				<dd class="mt-1 text-3xl font-semibold">{t.value}</dd>
			</div>
		{/each}
	</dl>

	<div class="grid gap-4 sm:grid-cols-2">
		<Card title="Users" description="Search, edit roles, deactivate or delete accounts.">
			<a href={resolve('/(admin)/admin/users')} class="text-sm">Manage users</a>
		</Card>
		<Card title="Audit logs" description="Security relevant events across the system.">
			<a href={resolve('/(admin)/admin/audit-logs')} class="text-sm">View audit logs</a>
		</Card>
	</div>
</div>
