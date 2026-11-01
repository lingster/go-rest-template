<script lang="ts">
	import Button from './Button.svelte';

	interface Props {
		page: number;
		perPage: number;
		total: number;
		onchange: (page: number) => void;
	}

	let { page, perPage, total, onchange }: Props = $props();

	const pages = $derived(Math.max(1, Math.ceil(total / perPage)));
	const from = $derived(total === 0 ? 0 : (page - 1) * perPage + 1);
	const to = $derived(Math.min(total, page * perPage));
</script>

<nav class="flex items-center justify-between gap-3 text-sm" aria-label="Pagination">
	<p class="text-slate-500 dark:text-slate-400">{from}-{to} of {total}</p>
	<div class="flex items-center gap-2">
		<Button variant="secondary" disabled={page <= 1} onclick={() => onchange(page - 1)}>
			Previous
		</Button>
		<span class="hidden sm:inline">Page {page} / {pages}</span>
		<Button variant="secondary" disabled={page >= pages} onclick={() => onchange(page + 1)}>
			Next
		</Button>
	</div>
</nav>
