<script lang="ts">
	import { AlertTriangle } from '@lucide/svelte';
	import { ApiError, authApi } from '$lib/api/auth';
	import { authStore } from '$lib/stores/auth.svelte';
	import { router } from '$lib/stores/router.svelte';
	import AuthShell from '$lib/components/layout/AuthShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	let confirming = $state(false);
	let confirmed = $state(false);
	let errorMsg = $state('');

	let restoreEmail = $state('');
	let restoreSending = $state(false);
	let restoreSentMsg = $state('');

	const token = $derived(router.queryParam('token'));

	async function handleConfirmDelete() {
		errorMsg = '';
		if (!token) {
			errorMsg = 'Deletion confirmation token is missing.';
			return;
		}
		confirming = true;
		try {
			await authApi.confirmAccountDeletion(token);
			authStore.setUser(null);
			confirmed = true;
		} catch (err) {
			errorMsg =
				err instanceof ApiError
					? err.message
					: 'Deletion confirmation token is invalid or expired.';
		} finally {
			confirming = false;
		}
	}

	async function handleSendRestoreLink(e: SubmitEvent) {
		e.preventDefault();
		errorMsg = '';
		restoreSentMsg = '';
		restoreSending = true;
		try {
			const res = await authApi.cancelAccountDeletion(restoreEmail);
			restoreSentMsg = res.message;
		} catch (err) {
			errorMsg = err instanceof ApiError ? err.message : 'Unable to send restore link.';
		} finally {
			restoreSending = false;
		}
	}
</script>

<AuthShell
	title={confirmed ? 'Account Scheduled for Deletion' : 'Confirm Account Deletion'}
	subtitle={confirmed
		? 'Your 30-day grace period has started.'
		: 'Final step before your account is scheduled for deletion.'}
>
	{#if errorMsg}
		<div
			role="alert"
			class="mb-4 rounded-md border-2 border-ink bg-coral/25 px-3.5 py-2.5 text-sm font-semibold text-ink"
		>
			{errorMsg}
		</div>
	{/if}

	{#if confirmed}
		<div class="space-y-4">
			<div class="rounded-md border-2 border-ink bg-coral/20 p-4 text-sm text-ink space-y-2">
				<p class="font-heading font-extrabold text-base">
					30-Day Grace Period Notice
				</p>
				<p>
					You have been signed out on all devices immediately. Your account and all owned
					licenses, products, and activations are now frozen and will be
					<strong>permanently deleted after 30 days</strong>.
				</p>
				<p>
					Changed your mind? You cannot sign in while deletion is pending, but you can cancel
					deletion and restore your account within 30 days by requesting a restore link below.
				</p>
			</div>

			{#if restoreSentMsg}
				<div class="rounded-md border-2 border-ink bg-yellow/40 p-3.5 text-sm font-semibold text-ink">
					{restoreSentMsg}
				</div>
			{:else}
				<form onsubmit={handleSendRestoreLink} class="space-y-3 pt-1">
					<label for="restore-email" class="block font-heading text-xs font-bold uppercase text-ink">
						Cancel Deletion & Send Restore Link
					</label>
					<input
						id="restore-email"
						type="email"
						required
						bind:value={restoreEmail}
						placeholder="you@company.com"
						class="w-full rounded-md border-2 border-ink bg-cream px-3.5 py-2 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt"
					/>
					<Button type="submit" variant="secondary" fullWidth disabled={restoreSending}>
						{restoreSending ? 'Sending Restore Link...' : 'Send Restore Link to My Email'}
					</Button>
				</form>
			{/if}

			<Button variant="ghost" fullWidth onclick={() => router.navigate('/login', true)}>
				Return to Sign In
			</Button>
		</div>
	{:else}
		<div class="space-y-4">
			<div class="rounded-md border-2 border-ink bg-coral/20 p-4 text-sm text-ink space-y-2">
				<div class="flex items-center gap-2 font-heading font-extrabold text-base">
					<AlertTriangle class="h-5 w-5 shrink-0" />
					<span>Permanent Deletion in 30 Days</span>
				</div>
				<ul class="list-disc pl-5 space-y-1 text-ink/90">
					<li>You will be logged out of all browsers and devices immediately.</li>
					<li>Two-factor authentication and trusted devices will be cleared.</li>
					<li>
						Your account and all owned <strong>licenses, products, and activations</strong> will
						be permanently deleted after 30 days.
					</li>
					<li>You can cancel deletion and restore your account within the 30-day window.</li>
				</ul>
			</div>

			{#if !token}
				<div class="rounded-md border-2 border-ink bg-coral/25 p-3.5 text-sm font-semibold text-ink">
					Missing confirmation token. Please open the link from your email.
				</div>
			{:else}
				<Button
					variant="coral"
					fullWidth
					disabled={confirming}
					onclick={handleConfirmDelete}
				>
					{confirming ? 'Deleting Account...' : 'Delete my account'}
				</Button>
			{/if}

			<Button
				variant="secondary"
				fullWidth
				onclick={() => router.navigate(authStore.user ? '/settings/security' : '/login', true)}
			>
				Cancel & Keep Account
			</Button>
		</div>
	{/if}
</AuthShell>
