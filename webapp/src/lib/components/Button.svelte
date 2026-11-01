<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLButtonAttributes } from 'svelte/elements';

	type Variant = 'primary' | 'secondary' | 'danger' | 'ghost';

	interface Props extends HTMLButtonAttributes {
		variant?: Variant;
		loading?: boolean;
		fullWidth?: boolean;
		children: Snippet;
	}

	let {
		variant = 'primary',
		loading = false,
		fullWidth = false,
		disabled = false,
		type = 'button',
		class: extra = '',
		children,
		...rest
	}: Props = $props();

	const variants: Record<Variant, string> = {
		primary:
			'bg-indigo-600 text-white hover:bg-indigo-500 focus-visible:outline-indigo-600 dark:bg-indigo-500 dark:hover:bg-indigo-400',
		secondary:
			'bg-white text-slate-900 ring-1 ring-slate-300 hover:bg-slate-50 dark:bg-slate-800 dark:text-slate-100 dark:ring-slate-600 dark:hover:bg-slate-700',
		danger: 'bg-red-600 text-white hover:bg-red-500 focus-visible:outline-red-600',
		ghost: 'text-slate-700 hover:bg-slate-100 dark:text-slate-200 dark:hover:bg-slate-800'
	};
</script>

<button
	{type}
	disabled={disabled || loading}
	aria-busy={loading}
	class="inline-flex items-center justify-center gap-2 rounded-md px-3.5 py-2 text-sm font-semibold shadow-xs focus-visible:outline-2 focus-visible:outline-offset-2 disabled:cursor-not-allowed disabled:opacity-60 {variants[
		variant
	]} {fullWidth ? 'w-full' : ''} {extra}"
	{...rest}
>
	{#if loading}
		<svg class="size-4 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
			<circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" class="opacity-25" />
			<path d="M4 12a8 8 0 018-8" stroke="currentColor" stroke-width="4" class="opacity-75" />
		</svg>
	{/if}
	{@render children()}
</button>
