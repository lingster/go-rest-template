<script lang="ts">
	import type { Snippet } from 'svelte';

	type Variant = 'error' | 'success' | 'info' | 'warning';

	interface Props {
		variant?: Variant;
		children: Snippet;
	}

	let { variant = 'info', children }: Props = $props();

	const styles: Record<Variant, string> = {
		error:
			'bg-red-50 text-red-800 ring-red-200 dark:bg-red-950 dark:text-red-200 dark:ring-red-900',
		success:
			'bg-green-50 text-green-800 ring-green-200 dark:bg-green-950 dark:text-green-200 dark:ring-green-900',
		info: 'bg-blue-50 text-blue-800 ring-blue-200 dark:bg-blue-950 dark:text-blue-200 dark:ring-blue-900',
		warning:
			'bg-amber-50 text-amber-900 ring-amber-200 dark:bg-amber-950 dark:text-amber-200 dark:ring-amber-900'
	};
</script>

<div
	role={variant === 'error' ? 'alert' : 'status'}
	class="rounded-md p-3 text-sm ring-1 {styles[variant]}"
>
	{@render children()}
</div>
