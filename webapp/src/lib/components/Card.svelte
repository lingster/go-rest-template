<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		title?: string;
		description?: string;
		children: Snippet;
		actions?: Snippet;
	}

	let { title, description, children, actions }: Props = $props();
</script>

<section
	class="rounded-xl bg-white p-5 shadow-xs ring-1 ring-slate-200 sm:p-6 dark:bg-slate-900 dark:ring-slate-800"
>
	{#if title || actions}
		<header class="mb-4 flex flex-wrap items-start justify-between gap-3">
			<div>
				{#if title}<h2 class="text-base font-semibold">{title}</h2>{/if}
				{#if description}
					<p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{description}</p>
				{/if}
			</div>
			{#if actions}<div class="flex gap-2">{@render actions()}</div>{/if}
		</header>
	{/if}
	{@render children()}
</section>
