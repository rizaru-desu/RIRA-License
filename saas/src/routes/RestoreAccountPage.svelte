<script lang="ts">
	import { ApiError, authApi } from '$lib/api/auth';
	import { router } from '$lib/stores/router.svelte';
	import AuthShell from '$lib/components/layout/AuthShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	let email = $state('');
	let loading = $state(false);
	let errorMsg = $state('');
	let successMsg = $state('');
	let restored = $state(false);

	const token = $derived(router.queryParam('token'));

	async function handleRequestCancel(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		successMsg = '';
		loading = true;
		try {
			const res = await authApi.cancelAccountDeletion(email);
			successMsg = res.message;
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Unable to send restore link.';
		} finally {
			loading = false;
		}
	}

	async function handleRestoreWithToken() {
		if (!token) return;
		errorMsg = '';
		successMsg = '';
		loading = true;
		try {
			const res = await authApi.restoreAccount(token);
			restored = true;
			successMsg = res.message;
		} catch (err) {
			errorMsg =
				err instanceof ApiError ? err.message : 'Restore token is invalid or has expired.';
		} finally {
			loading = false;
		}
	}
</script>

<AuthShell
	title="Restore Your Account"
	subtitle="Accounts scheduled for deletion can be restored within the 30-day grace period."
>
	{#if errorMsg}
		<div
			role="alert"
			class="mb-4 rounded-md border-2 border-ink bg-coral/25 px-3.5 py-2.5 text-sm font-semibold text-ink"
		>
			{errorMsg}
		</div>
	{/if}

	{#if successMsg}
		<div class="space-y-4">
			<div class="rounded-md border-2 border-ink bg-yellow/40 p-4 text-sm font-semibold text-ink">
				{successMsg}
			</div>
			{#if restored}
				<Button fullWidth onclick={() => router.navigate('/login', true)}>
					Go to Sign In
				</Button>
			{/if}
		</div>
	{:else if token}
		<div class="space-y-4">
			<p class="text-sm text-ink/85">
				Click below to cancel scheduled deletion and restore your RiraLicense account to active
				status.
			</p>
			<Button fullWidth disabled={loading} onclick={handleRestoreWithToken}>
				{loading ? 'Restoring Account...' : 'Restore My Account'}
			</Button>
		</div>
	{:else}
		<form onsubmit={handleRequestCancel} class="space-y-4">
			<div>
				<label for="cancel-del-email" class="mb-1.5 block font-heading text-sm font-bold text-ink">
					Account Email
				</label>
				<input
					id="cancel-del-email"
					type="email"
					required
					bind:value={email}
					placeholder="you@company.com"
					class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2.5 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
				/>
			</div>
			<Button type="submit" fullWidth disabled={loading}>
				{loading ? 'Sending...' : 'Send Restore Link'}
			</Button>
		</form>
	{/if}

	<p class="mt-6 text-center text-sm">
		<button
			type="button"
			onclick={() => router.navigate('/login')}
			class="font-heading font-bold text-cobalt underline cursor-pointer"
		>
			Back to sign in
		</button>
	</p>
</AuthShell>
