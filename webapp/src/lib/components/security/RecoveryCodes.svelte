<script lang="ts">
	import Alert from '../Alert.svelte';
	import Button from '../Button.svelte';

	interface Props {
		codes: string[];
		ondismiss: () => void;
	}

	let { codes, ondismiss }: Props = $props();
	let copied = $state(false);

	async function copy() {
		try {
			await navigator.clipboard.writeText(codes.join('\n'));
			copied = true;
		} catch {
			copied = false;
		}
	}
</script>

<div class="space-y-3">
	<Alert variant="warning">
		Save these recovery codes somewhere safe. Each can be used once if you lose your authenticator.
		They will not be shown again.
	</Alert>
	<ul
		class="grid grid-cols-2 gap-2 rounded-md bg-slate-100 p-3 font-mono text-sm dark:bg-slate-800"
	>
		{#each codes as code (code)}
			<li>{code}</li>
		{/each}
	</ul>
	<div class="flex gap-2">
		<Button variant="secondary" onclick={copy}>{copied ? 'Copied' : 'Copy'}</Button>
		<Button onclick={ondismiss}>I have saved them</Button>
	</div>
</div>
