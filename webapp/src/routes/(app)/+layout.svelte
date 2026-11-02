<script lang="ts">
	import Seo from '#lib/components/Seo.svelte';
	import AppShell from '#lib/components/AppShell.svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<AppShell>{@render children()}</AppShell>

<Seo noindex />
