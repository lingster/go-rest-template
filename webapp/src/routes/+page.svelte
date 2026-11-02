<script lang="ts">
	import { resolve } from '$app/paths';
	import Seo from '#lib/components/Seo.svelte';
	import { MAIN_SITE_URL, SITE_NAME, TAGLINE } from '#lib/seo.ts';

	const features = [
		{
			title: 'Secure authentication',
			body: 'Email and password, email verification and password reset.'
		},
		{ title: 'Two-factor and passkeys', body: 'TOTP with recovery codes, plus WebAuthn passkeys.' },
		{ title: 'Admin console', body: 'Manage users, roles, sessions and review the audit log.' }
	];
</script>

<Seo
	title="{SITE_NAME} – {TAGLINE}"
	description="Sign in to {SITE_NAME} to manage your account and security settings, including two-factor authentication and passkeys."
	canonical={MAIN_SITE_URL}
/>

<header class="mx-auto flex max-w-5xl items-center justify-between px-4 py-4">
	<span class="text-lg font-semibold">{SITE_NAME}</span>
	<nav class="flex items-center gap-4 text-sm">
		<a href={resolve('/(auth)/login')}>Sign in</a>
		<a
			href={resolve('/(auth)/register')}
			class="rounded-md bg-indigo-600 px-3.5 py-2 font-semibold text-white hover:bg-indigo-500"
		>
			Get started
		</a>
	</nav>
</header>

<main class="mx-auto max-w-5xl px-4 py-16 sm:py-24">
	<div class="text-center">
		<h1 class="text-4xl font-bold tracking-tight sm:text-6xl">Ship your SaaS faster</h1>
		<p class="mx-auto mt-6 max-w-2xl text-lg text-slate-600 dark:text-slate-300">
			Authentication, security and administration are already done. Focus on what makes your product
			unique.
		</p>
		<div class="mt-8 flex justify-center gap-3">
			<a
				href={resolve('/(auth)/register')}
				class="rounded-md bg-indigo-600 px-5 py-2.5 text-sm font-semibold text-white hover:bg-indigo-500"
			>
				Create an account
			</a>
			<a
				href={resolve('/(auth)/login')}
				class="rounded-md bg-white px-5 py-2.5 text-sm font-semibold text-slate-900 ring-1 ring-slate-300 hover:bg-slate-50 dark:bg-slate-800 dark:text-slate-100 dark:ring-slate-600"
			>
				Sign in
			</a>
		</div>
	</div>

	<ul class="mt-16 grid gap-6 sm:grid-cols-3">
		{#each features as f (f.title)}
			<li
				class="rounded-xl bg-white p-5 ring-1 ring-slate-200 dark:bg-slate-900 dark:ring-slate-800"
			>
				<h2 class="font-semibold">{f.title}</h2>
				<p class="mt-2 text-sm text-slate-600 dark:text-slate-400">{f.body}</p>
			</li>
		{/each}
	</ul>
</main>
