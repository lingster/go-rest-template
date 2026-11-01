<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLInputAttributes } from 'svelte/elements';

	interface Props extends Omit<HTMLInputAttributes, 'value'> {
		label: string;
		id: string;
		value?: string;
		error?: string;
		hint?: string;
		trailing?: Snippet;
	}

	let {
		label,
		id,
		value = $bindable(''),
		error,
		hint,
		type = 'text',
		trailing,
		...rest
	}: Props = $props();
</script>

<div>
	<label for={id} class="block text-sm font-medium">{label}</label>
	<div class="relative mt-1">
		<input
			{id}
			{type}
			bind:value
			aria-invalid={error ? 'true' : undefined}
			aria-describedby={error ? `${id}-error` : hint ? `${id}-hint` : undefined}
			class="block w-full rounded-md border-0 bg-white px-3 py-2 text-sm shadow-xs ring-1 placeholder:text-slate-400 focus:ring-2 focus:ring-indigo-600 focus:outline-none disabled:opacity-60 dark:bg-slate-800 dark:focus:ring-indigo-400 {error
				? 'ring-red-500'
				: 'ring-slate-300 dark:ring-slate-600'} {trailing ? 'pr-16' : ''}"
			{...rest}
		/>
		{#if trailing}
			<div class="absolute inset-y-0 right-0 flex items-center pr-2">{@render trailing()}</div>
		{/if}
	</div>
	{#if error}
		<p id="{id}-error" class="mt-1 text-sm text-red-600 dark:text-red-400">{error}</p>
	{:else if hint}
		<p id="{id}-hint" class="mt-1 text-xs text-slate-500 dark:text-slate-400">{hint}</p>
	{/if}
</div>
