<script lang="ts">
	import { ApiError, authApi } from '$lib/api/auth';
	import { router } from '$lib/stores/router.svelte';
	import AuthShell from '$lib/components/layout/AuthShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	let email = $state('');
	let submitting = $state(false);
	let errorMsg = $state('');
	let successMsg = $state('');

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		successMsg = '';
		submitting = true;
		try {
			const res = await authApi.forgotPassword(email);
			successMsg = res.message;
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Unable to process request.';
		} finally {
			submitting = false;
		}
	}
</script>

<AuthShell title="Reset your password" subtitle="We'll email you a 1-hour password reset link.">
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
			<Button variant="secondary" fullWidth onclick={() => router.navigate('/login')}>
				Return to Sign In
			</Button>
		</div>
	{:else}
		<form onsubmit={handleSubmit} class="space-y-4">
			<div>
				<label for="forgot-email" class="mb-1.5 block font-heading text-sm font-bold text-ink">
					Account Email
				</label>
				<input
					id="forgot-email"
					type="email"
					required
					bind:value={email}
					placeholder="you@company.com"
					class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2.5 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
				/>
			</div>
			<Button type="submit" fullWidth disabled={submitting}>
				{submitting ? 'Sending...' : 'Send Reset Link'}
			</Button>
		</form>

		<p class="mt-6 text-center text-sm">
			<button
				type="button"
				onclick={() => router.navigate('/login')}
				class="font-heading font-bold text-cobalt underline cursor-pointer"
			>
				Back to sign in
			</button>
		</p>
	{/if}
</AuthShell>
