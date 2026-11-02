<script lang="ts">
	import Seo from '#lib/components/Seo.svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<Seo noindex />
{@render children()}
