<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import * as adminApi from '#lib/api/admin.ts';
	import type { Paginated, User } from '#lib/api/types.ts';
	import UserActions from '#lib/components/admin/UserActions.svelte';
	import Alert from '#lib/components/Alert.svelte';
	import Button from '#lib/components/Button.svelte';
	import Card from '#lib/components/Card.svelte';
	import FormField from '#lib/components/FormField.svelte';
	import Pagination from '#lib/components/Pagination.svelte';
	import { errorMessage } from '#lib/forms.ts';

	const PER_PAGE = 20;

	let result = $state<Paginated<User> | null>(null);
	let query = $state('');
	let appliedQuery = $state('');
	let page = $state(1);
	let loading = $state(true);
	let error = $state('');
	let notice = $state('');

	async function load() {
		loading = true;
		try {
			result = await adminApi.listUsers({ page, per_page: PER_PAGE, q: appliedQuery });
			error = '';
		} catch (err) {
			error = errorMessage(err);
		} finally {
			loading = false;
		}
	}

	onMount(load);

	function search(event: SubmitEvent) {
		event.preventDefault();
		appliedQuery = query.trim();
		page = 1;
		void load();
	}

	function goToPage(next: number) {
		page = next;
		void load();
	}

	function replaceUser(u: User) {
		if (result) result = { ...result, items: result.items.map((x) => (x.id === u.id ? u : x)) };
	}

	async function removeUser() {
		notice = 'User deleted.';
		await load();
		// Step back if we deleted the last row of the final page.
		if (result && result.items.length === 0 && page > 1) goToPage(page - 1);
	}
</script>

<svelte:head><title>Users</title></svelte:head>

<h1 class="text-2xl font-bold">Users</h1>

<div class="mt-6 space-y-4">
	{#if error}<Alert variant="error">{error}</Alert>{/if}
	{#if notice}<Alert variant="success">{notice}</Alert>{/if}

	<form class="flex items-end gap-2" onsubmit={search} role="search">
		<div class="flex-1">
			<FormField
				id="q"
				label="Search"
				type="search"
				placeholder="Name or email"
				bind:value={query}
			/>
		</div>
		<Button type="submit">Search</Button>
	</form>

	<Card>
		{#if loading && !result}
			<p class="text-sm text-slate-500" role="status">Loading...</p>
		{:else if result && result.items.length === 0}
			<p class="text-sm text-slate-500">No users found.</p>
		{:else if result}
			<ul class="divide-y divide-slate-200 dark:divide-slate-800" aria-busy={loading}>
				{#each result.items as u (u.id)}
					<li class="space-y-2 py-4 first:pt-0 last:pb-0">
						<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
							<a href={resolve('/(admin)/admin/users/[id]', { id: u.id })} class="font-medium"
								>{u.name}</a
							>
							<span class="text-sm break-all text-slate-500 dark:text-slate-400">{u.email}</span>
							<span class="rounded-full bg-slate-100 px-2 py-0.5 text-xs dark:bg-slate-800"
								>{u.role}</span
							>
							{#if !u.is_active}
								<span
									class="rounded-full bg-red-100 px-2 py-0.5 text-xs text-red-800 dark:bg-red-950 dark:text-red-200"
									>inactive</span
								>
							{/if}
							{#if !u.email_verified}
								<span
									class="rounded-full bg-amber-100 px-2 py-0.5 text-xs text-amber-900 dark:bg-amber-950 dark:text-amber-200"
									>unverified</span
								>
							{/if}
						</div>
						<UserActions
							user={u}
							onupdate={replaceUser}
							ondelete={removeUser}
							onerror={(m) => (error = m)}
							onnotice={(m) => (notice = m)}
						/>
					</li>
				{/each}
			</ul>
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
