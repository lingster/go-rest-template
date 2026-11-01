<script lang="ts">
	import type { Snippet } from 'svelte';
	import { resolve } from '$app/paths';

	interface Props {
		title: string;
		subtitle?: string;
		children: Snippet;
		footer?: Snippet;
	}

	let { title, subtitle, children, footer }: Props = $props();
</script>

<div class="flex min-h-screen flex-col justify-center px-4 py-10">
	<div class="mx-auto w-full max-w-md">
		<a href={resolve('/')} class="block text-center text-lg font-semibold">SaaS Starter</a>
		<h1 class="mt-6 text-center text-2xl font-bold tracking-tight">{title}</h1>
		{#if subtitle}
			<p class="mt-2 text-center text-sm text-slate-500 dark:text-slate-400">{subtitle}</p>
		{/if}
		<div
			class="mt-6 space-y-4 rounded-xl bg-white p-6 shadow-xs ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
		>
			{@render children()}
		</div>
		{#if footer}
			<div class="mt-4 text-center text-sm text-slate-500 dark:text-slate-400">
				{@render footer()}
			</div>
		{/if}
	</div>
</div>
