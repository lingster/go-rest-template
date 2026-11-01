<script lang="ts">
	import { onMount } from 'svelte';
	import * as meApi from '../../api/me.ts';
	import type { Passkey } from '../../api/types.ts';
	import { errorMessage, formatDate } from '../../forms.ts';
	import { isPasskeyCancelled, registerPasskey } from '../../passkeys.ts';
	import { session } from '../../stores/session.svelte.ts';
	import Alert from '../Alert.svelte';
	import Button from '../Button.svelte';
	import Card from '../Card.svelte';
	import FormField from '../FormField.svelte';

	let passkeys = $state<Passkey[]>([]);
	let name = $state('');
	let loading = $state(true);
	let adding = $state(false);
	let error = $state('');
	let deleting = $state<string | null>(null);

	const supported = typeof window !== 'undefined' && 'PublicKeyCredential' in window;

	async function load() {
		try {
			passkeys = await meApi.listPasskeys();
		} catch (err) {
			error = errorMessage(err);
		} finally {
			loading = false;
		}
	}

	onMount(load);

	async function add(event: SubmitEvent) {
		event.preventDefault();
		adding = true;
		error = '';
		try {
			const created = await registerPasskey(name.trim() || 'My passkey');
			passkeys = [...passkeys, created];
			name = '';
			await session.refreshUser();
		} catch (err) {
			if (!isPasskeyCancelled(err)) error = errorMessage(err);
		} finally {
			adding = false;
		}
	}

	async function remove(p: Passkey) {
		if (!confirm(`Delete passkey "${p.name}"?`)) return;
		deleting = p.id;
		error = '';
		try {
			await meApi.deletePasskey(p.id);
			passkeys = passkeys.filter((x) => x.id !== p.id);
			await session.refreshUser();
		} catch (err) {
			error = errorMessage(err);
		} finally {
			deleting = null;
		}
	}
</script>

<Card title="Passkeys" description="Sign in with your fingerprint, face or a security key.">
	<div class="space-y-4">
		{#if error}<Alert variant="error">{error}</Alert>{/if}

		{#if loading}
			<p class="text-sm text-slate-500" role="status">Loading...</p>
		{:else if passkeys.length === 0}
			<p class="text-sm text-slate-500 dark:text-slate-400">No passkeys registered.</p>
		{:else}
			<ul class="divide-y divide-slate-200 dark:divide-slate-800">
				{#each passkeys as p (p.id)}
					<li class="flex flex-wrap items-center justify-between gap-2 py-3">
						<div>
							<p class="text-sm font-medium">{p.name}</p>
							<p class="text-xs text-slate-500 dark:text-slate-400">
								Added {formatDate(p.created_at)} - last used {formatDate(p.last_used_at)}
							</p>
						</div>
						<Button variant="danger" loading={deleting === p.id} onclick={() => remove(p)}>
							Delete
						</Button>
					</li>
				{/each}
			</ul>
		{/if}

		{#if supported}
			<form class="flex flex-col gap-3 sm:flex-row sm:items-end" onsubmit={add}>
				<div class="flex-1">
					<FormField
						id="passkey-name"
						label="Passkey name"
						placeholder="MacBook"
						bind:value={name}
					/>
				</div>
				<Button type="submit" loading={adding}>Add passkey</Button>
			</form>
		{:else}
			<Alert variant="info">This browser does not support passkeys.</Alert>
		{/if}
	</div>
</Card>
