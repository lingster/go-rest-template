<script lang="ts">
	import * as meApi from '#lib/api/me.ts';
	import Alert from '#lib/components/Alert.svelte';
	import Button from '#lib/components/Button.svelte';
	import Card from '#lib/components/Card.svelte';
	import FormField from '#lib/components/FormField.svelte';
	import PasswordField from '#lib/components/PasswordField.svelte';
	import { errorMessage, fieldErrors, passwordPolicyError } from '#lib/forms.ts';
	import { session } from '#lib/stores/session.svelte.ts';

	let name = $state(session.user?.name ?? '');
	let nameLoading = $state(false);
	let nameError = $state('');
	let nameOk = $state(false);

	let current = $state('');
	let next = $state('');
	let confirm = $state('');
	let pwLoading = $state(false);
	let pwError = $state('');
	let pwErrors = $state<Record<string, string>>({});
	let pwOk = $state('');

	async function saveName(event: SubmitEvent) {
		event.preventDefault();
		nameLoading = true;
		nameError = '';
		nameOk = false;
		try {
			session.setUser(await meApi.updateMe({ name: name.trim() }));
			nameOk = true;
		} catch (err) {
			nameError = errorMessage(err);
		} finally {
			nameLoading = false;
		}
	}

	async function changePassword(event: SubmitEvent) {
		event.preventDefault();
		pwError = '';
		pwOk = '';
		pwErrors = {};
		const policy = passwordPolicyError(next);
		if (policy) {
			pwErrors = { new_password: policy };
			return;
		}
		if (next !== confirm) {
			pwErrors = { confirm: 'Passwords do not match.' };
			return;
		}
		pwLoading = true;
		try {
			pwOk = (await meApi.changePassword({ current_password: current, new_password: next }))
				.message;
			current = next = confirm = '';
		} catch (err) {
			pwError = errorMessage(err);
			pwErrors = fieldErrors(err);
		} finally {
			pwLoading = false;
		}
	}
</script>

<svelte:head><title>Profile</title></svelte:head>

<h1 class="text-2xl font-bold">Profile</h1>

<div class="mt-6 grid gap-6 lg:grid-cols-2">
	<Card title="Your details">
		<form class="space-y-4" onsubmit={saveName}>
			{#if nameError}<Alert variant="error">{nameError}</Alert>{/if}
			{#if nameOk}<Alert variant="success">Profile updated.</Alert>{/if}
			<FormField id="email" label="Email" value={session.user?.email ?? ''} disabled />
			<FormField id="name" label="Name" autocomplete="name" required bind:value={name} />
			<Button type="submit" loading={nameLoading}>Save</Button>
		</form>
	</Card>

	<Card title="Change password">
		<form class="space-y-4" onsubmit={changePassword} novalidate>
			{#if pwError}<Alert variant="error">{pwError}</Alert>{/if}
			{#if pwOk}<Alert variant="success">{pwOk}</Alert>{/if}
			<PasswordField
				id="current"
				label="Current password"
				autocomplete="current-password"
				required
				bind:value={current}
				error={pwErrors.current_password}
			/>
			<PasswordField
				id="new"
				label="New password"
				autocomplete="new-password"
				required
				hint="8 to 128 characters."
				bind:value={next}
				error={pwErrors.new_password}
			/>
			<PasswordField
				id="confirm"
				label="Confirm new password"
				autocomplete="new-password"
				required
				bind:value={confirm}
				error={pwErrors.confirm}
			/>
			<Button type="submit" loading={pwLoading}>Change password</Button>
		</form>
	</Card>
</div>
