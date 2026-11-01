import { goto } from '$app/navigation';

/**
 * Navigate to an already-validated same-origin path (e.g. a `?redirect=` target that has
 * been through `safeRedirect`). Static routes should use `goto(resolve(...))` instead.
 */
export function gotoPath(path: string, opts?: { replaceState?: boolean }): Promise<void> {
	// eslint-disable-next-line svelte/no-navigation-without-resolve -- dynamic, validated path
	return goto(path, opts);
}
