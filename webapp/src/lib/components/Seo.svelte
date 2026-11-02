<script lang="ts">
	interface Props {
		title?: string;
		description?: string;
		/** Absolute canonical URL. */
		canonical?: string;
		/** Keep the page out of search results (authenticated areas). */
		noindex?: boolean;
	}

	let { title, description, canonical, noindex = false }: Props = $props();
</script>

<svelte:head>
	{#if title}<title>{title}</title>{/if}
	{#if description}<meta name="description" content={description} />{/if}
	{#if canonical}<link rel="canonical" href={canonical} />{/if}
	{#if noindex}<meta name="robots" content="noindex, nofollow" />{/if}
</svelte:head>
