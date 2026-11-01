<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';
	import FormField from './FormField.svelte';

	interface Props extends Omit<HTMLInputAttributes, 'value' | 'type'> {
		label: string;
		id: string;
		value?: string;
		error?: string;
		hint?: string;
	}

	let { value = $bindable(''), ...rest }: Props = $props();
	let visible = $state(false);
</script>

<FormField {...rest} bind:value type={visible ? 'text' : 'password'}>
	{#snippet trailing()}
		<button
			type="button"
			class="rounded px-2 py-1 text-xs font-medium text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100"
			aria-pressed={visible}
			onclick={() => (visible = !visible)}
		>
			{visible ? 'Hide' : 'Show'}
		</button>
	{/snippet}
</FormField>
