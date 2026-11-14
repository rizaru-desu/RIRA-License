<script lang="ts">
	import { ApiError, authApi } from '$lib/api/auth';
	import { router } from '$lib/stores/router.svelte';
	import AuthShell from '$lib/components/layout/AuthShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	let status = $state<'idle' | 'verifying' | 'verified' | 'error'>('idle');
	let message = $state('');
	let resendEmail = $state(router.queryParam('email'));
	let resending = $state(false);
	let resendMsg = $state('');

	$effect(() => {
		const token = router.queryParam('token');
		if (!token) return;

		status = 'verifying';
		authApi
			.verifyEmail(token)
			.then((res) => {
				status = 'verified';
				message = res.message;
			})
			.catch((err) => {
				status = 'error';
				message = err instanceof ApiError ? err.message : 'Verification link is invalid or expired.';
			});
	});

	async function handleResend(e: SubmitEvent) {
		e.preventDefault();
		resending = true;
		resendMsg = '';
		try {
			const res = await authApi.resendVerification(resendEmail);
			resendMsg = res.message;
		} catch (err) {
			resendMsg = err instanceof ApiError ? err.message : 'Could not resend verification email.';
		} finally {
			resending = false;
		}
	}
</script>

<AuthShell title="Verify your email" subtitle="Confirm your email address to activate sign-in.">
	{#if status === 'verifying'}
		<p class="font-mono text-sm font-bold text-ink">Verifying token...</p>
	{:else if status === 'verified'}
		<div class="space-y-4">
			<div class="rounded-md border-2 border-ink bg-yellow/40 p-4 text-sm font-medium text-ink">
				{message}
			</div>
			<Button fullWidth onclick={() => router.navigate('/login', true)}>Sign In Now</Button>
		</div>
	{:else}
		{#if status === 'error'}
			<div
				role="alert"
				class="mb-4 rounded-md border-2 border-ink bg-coral/25 px-3.5 py-2.5 text-sm font-medium text-ink"
			>
				{message}
			</div>
		{/if}

		{#if resendMsg}
			<div class="mb-4 rounded-md border-2 border-ink bg-yellow/40 px-3.5 py-2.5 text-sm font-medium text-ink">
				{resendMsg}
			</div>
		{/if}

		<form onsubmit={handleResend} class="space-y-4">
			<div>
				<label for="verify-email-input" class="mb-1.5 block font-heading text-sm font-bold text-ink">
					Resend verification email
				</label>
				<input
					id="verify-email-input"
					type="email"
					required
					bind:value={resendEmail}
					placeholder="you@company.com"
					class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2.5 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
				/>
			</div>
			<Button type="submit" fullWidth disabled={resending}>
				{resending ? 'Sending...' : 'Send Verification Link'}
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

