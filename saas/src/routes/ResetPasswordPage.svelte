<script lang="ts">
	import { ApiError, authApi } from '$lib/api/auth';
	import { router } from '$lib/stores/router.svelte';
	import AuthShell from '$lib/components/layout/AuthShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import PasswordInput from '$lib/components/ui/PasswordInput.svelte';

	let newPassword = $state('');
	let confirmPassword = $state('');
	let submitting = $state(false);
	let errorMsg = $state('');
	let successMsg = $state('');

	async function handleReset(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		const token = router.queryParam('token');
		if (!token) {
			errorMsg = 'Missing password reset token.';
			return;
		}
		if (newPassword.length < 12) {
			errorMsg = 'Password must be at least 12 characters.';
			return;
		}
		if (newPassword !== confirmPassword) {
			errorMsg = 'Passwords do not match.';
			return;
		}

		submitting = true;
		try {
			const res = await authApi.resetPassword(token, newPassword);
			successMsg = res.message;
			newPassword = '';
			confirmPassword = '';
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Unable to reset password.';
		} finally {
			submitting = false;
		}
	}
</script>

<AuthShell title="Choose a new password" subtitle="Must be at least 12 characters and not match your last 5 passwords.">
	{#if errorMsg}
		<div
			role="alert"
			class="mb-4 rounded-md border-2 border-ink bg-coral/25 px-3.5 py-2.5 text-sm font-medium text-ink"
		>
			{errorMsg}
		</div>
	{/if}

	{#if successMsg}
		<div class="space-y-4">
			<div class="rounded-md border-2 border-ink bg-yellow/40 p-4 text-sm font-medium text-ink">
				{successMsg}
			</div>
			<Button fullWidth onclick={() => router.navigate('/login', true)}>Sign In</Button>
		</div>
	{:else}
		<form onsubmit={handleReset} class="space-y-4">
			<div>
				<label for="reset-pw" class="mb-1.5 block font-heading text-sm font-bold text-ink">
					New Password
				</label>
				<PasswordInput
					id="reset-pw"
					autocomplete="new-password"
					required
					minlength={12}
					bind:value={newPassword}
				/>
			</div>
			<div>
				<label for="reset-pw-confirm" class="mb-1.5 block font-heading text-sm font-bold text-ink">
					Confirm Password
				</label>
				<PasswordInput
					id="reset-pw-confirm"
					autocomplete="new-password"
					required
					minlength={12}
					bind:value={confirmPassword}
				/>
			</div>
			<Button type="submit" fullWidth disabled={submitting}>
				{submitting ? 'Updating...' : 'Reset Password'}
			</Button>
		</form>
	{/if}
</AuthShell>

