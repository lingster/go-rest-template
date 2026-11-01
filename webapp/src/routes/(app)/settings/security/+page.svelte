<script lang="ts">
	import PasskeySection from '#lib/components/security/PasskeySection.svelte';
	import TotpSection from '#lib/components/security/TotpSection.svelte';
</script>

<svelte:head><title>Security</title></svelte:head>

<h1 class="text-2xl font-bold">Security</h1>

<div class="mt-6 space-y-6">
	<TotpSection />
	<PasskeySection />
</div>
