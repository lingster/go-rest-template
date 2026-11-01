<script lang="ts">
	import * as adminApi from '../../api/admin.ts';
	import type { Role, User } from '../../api/types.ts';
	import { errorMessage } from '../../forms.ts';
	import { session } from '../../stores/session.svelte.ts';
	import Button from '../Button.svelte';

	interface Props {
		user: User;
		onupdate: (user: User) => void;
		ondelete: (id: string) => void;
		onerror: (message: string) => void;
		onnotice?: (message: string) => void;
	}

	let { user, onupdate, ondelete, onerror, onnotice }: Props = $props();

	let busy = $state(false);
	const isSelf = $derived(session.user?.id === user.id);

	async function run(action: () => Promise<void>) {
		busy = true;
		onerror('');
		try {
			await action();
		} catch (err) {
			onerror(errorMessage(err));
		} finally {
			busy = false;
		}
	}

	const patch = (body: Parameters<typeof adminApi.updateUser>[1]) =>
		run(async () => onupdate(await adminApi.updateUser(user.id, body)));

	const setRole = (event: Event) =>
		patch({ role: (event.currentTarget as HTMLSelectElement).value as Role });

	const revoke = () =>
		run(async () => {
			await adminApi.revokeSessions(user.id);
			onnotice?.(`Sessions revoked for ${user.email}.`);
		});

	function remove() {
		if (!confirm(`Delete ${user.email}? This cannot be undone.`)) return;
		void run(async () => {
			await adminApi.deleteUser(user.id);
			ondelete(user.id);
		});
	}
</script>

<div class="flex flex-wrap items-center gap-2">
	<select
		aria-label="Role for {user.email}"
		class="rounded-md bg-white px-2 py-1.5 text-sm ring-1 ring-slate-300 dark:bg-slate-800 dark:ring-slate-600"
		value={user.role}
		disabled={busy || isSelf}
		onchange={setRole}
	>
		<option value="user">user</option>
		<option value="admin">admin</option>
	</select>
	<Button
		variant="secondary"
		disabled={busy || isSelf}
		onclick={() => patch({ is_active: !user.is_active })}
	>
		{user.is_active ? 'Deactivate' : 'Activate'}
	</Button>
	{#if !user.email_verified}
		<Button variant="secondary" disabled={busy} onclick={() => patch({ email_verified: true })}>
			Verify email
		</Button>
	{/if}
	<Button variant="secondary" disabled={busy} onclick={revoke}>Revoke sessions</Button>
	<Button variant="danger" disabled={busy || isSelf} onclick={remove}>Delete</Button>
</div>
