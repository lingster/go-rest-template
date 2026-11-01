<script lang="ts">
	import { onMount, type Snippet } from 'svelte';
	import { goto } from '$app/navigation';
	import { gotoPath } from '../navigation.ts';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { session } from '../stores/session.svelte.ts';
	import Button from './Button.svelte';

	interface Props {
		requireAdmin?: boolean;
		children: Snippet;
	}

	let { requireAdmin = false, children }: Props = $props();

	const links = $derived([
		{ href: resolve('/(app)/dashboard'), label: 'Dashboard' },
		{ href: resolve('/(app)/settings/profile'), label: 'Profile' },
		{ href: resolve('/(app)/settings/security'), label: 'Security' },
		...(session.isAdmin ? [{ href: resolve('/(admin)/admin'), label: 'Admin' }] : [])
	]);

	const allowed = $derived(session.isAuthenticated && (!requireAdmin || session.isAdmin));

	onMount(() => {
		void session.init();
	});

	$effect(() => {
		if (!session.ready) return;
		if (!session.isAuthenticated) {
			const back = encodeURIComponent(page.url.pathname + page.url.search);
			void gotoPath(`${resolve('/(auth)/login')}?redirect=${back}`, { replaceState: true });
		} else if (requireAdmin && !session.isAdmin) {
			void goto(resolve('/(app)/dashboard'), { replaceState: true });
		}
	});

	async function signOut() {
		await session.signOut();
		await goto(resolve('/(auth)/login'));
	}
</script>

{#if allowed}
	<header class="border-b border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
		<div class="mx-auto flex max-w-5xl flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3">
			<a href={resolve('/(app)/dashboard')} class="font-semibold">SaaS Starter</a>
			<nav class="flex flex-1 flex-wrap gap-1 text-sm" aria-label="Main">
				{#each links as link (link.href)}
					<a
						href={link.href}
						aria-current={page.url.pathname.startsWith(link.href) ? 'page' : undefined}
						class="rounded-md px-3 py-1.5 hover:bg-slate-100 aria-[current=page]:bg-slate-100 aria-[current=page]:font-medium dark:hover:bg-slate-800 dark:aria-[current=page]:bg-slate-800"
					>
						{link.label}
					</a>
				{/each}
			</nav>
			<div class="flex items-center gap-3 text-sm">
				<span class="hidden text-slate-500 sm:inline dark:text-slate-400"
					>{session.user?.email}</span
				>
				<Button variant="secondary" onclick={signOut}>Sign out</Button>
			</div>
		</div>
	</header>
	<main class="mx-auto max-w-5xl px-4 py-6 sm:py-8">
		{@render children()}
	</main>
{:else}
	<p class="p-8 text-center text-sm text-slate-500" role="status">Loading...</p>
{/if}
