<script lang="ts">
	import { onMount } from 'svelte';
	import { page as routePage } from '$app/state';
	import * as adminApi from '#lib/api/admin.ts';
	import type { AuditLog, Paginated } from '#lib/api/types.ts';
	import Alert from '#lib/components/Alert.svelte';
	import Card from '#lib/components/Card.svelte';
	import Pagination from '#lib/components/Pagination.svelte';
	import { errorMessage, formatDate } from '#lib/forms.ts';

	const PER_PAGE = 20;

	let result = $state<Paginated<AuditLog> | null>(null);
	let page = $state(1);
	let loading = $state(true);
	let error = $state('');

	const userId = $derived(routePage.url.searchParams.get('user_id') ?? undefined);

	async function load() {
		loading = true;
		try {
			result = await adminApi.listAuditLogs({ page, per_page: PER_PAGE, user_id: userId });
			error = '';
		} catch (err) {
			error = errorMessage(err);
		} finally {
			loading = false;
		}
	}

	onMount(load);

	function goToPage(next: number) {
		page = next;
		void load();
	}
</script>

<svelte:head><title>Audit logs</title></svelte:head>

<h1 class="text-2xl font-bold">Audit logs</h1>
{#if userId}
	<p class="mt-1 text-sm text-slate-500 dark:text-slate-400">Filtered by user {userId}</p>
{/if}

<div class="mt-6 space-y-4">
	{#if error}<Alert variant="error">{error}</Alert>{/if}

	<Card>
		{#if loading && !result}
			<p class="text-sm text-slate-500" role="status">Loading...</p>
		{:else if result && result.items.length === 0}
			<p class="text-sm text-slate-500">No audit events.</p>
		{:else if result}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-sm" aria-busy={loading}>
					<thead class="text-xs text-slate-500 uppercase dark:text-slate-400">
						<tr>
							<th class="py-2 pr-4">When</th>
							<th class="py-2 pr-4">Action</th>
							<th class="py-2 pr-4">User</th>
							<th class="py-2 pr-4">Actor</th>
							<th class="py-2 pr-4">IP</th>
							<th class="py-2">Details</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-slate-200 dark:divide-slate-800">
						{#each result.items as log (log.id)}
							<tr>
								<td class="py-2 pr-4 whitespace-nowrap">{formatDate(log.created_at)}</td>
								<td class="py-2 pr-4 font-medium whitespace-nowrap">{log.action}</td>
								<td class="py-2 pr-4 font-mono text-xs">{log.user_id ?? '-'}</td>
								<td class="py-2 pr-4 font-mono text-xs">{log.actor_id ?? '-'}</td>
								<td class="py-2 pr-4 whitespace-nowrap">{log.ip}</td>
								<td class="py-2 font-mono text-xs break-all">
									{log.metadata ? JSON.stringify(log.metadata) : ''}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</Card>

	{#if result}
		<Pagination
			page={result.page}
			perPage={result.per_page}
			total={result.total}
			onchange={goToPage}
		/>
	{/if}
</div>
