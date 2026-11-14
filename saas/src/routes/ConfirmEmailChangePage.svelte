<script lang="ts">
	import { ApiError, authApi } from '$lib/api/auth';
	import { authStore } from '$lib/stores/auth.svelte';
	import { router } from '$lib/stores/router.svelte';
	import AuthShell from '$lib/components/layout/AuthShell.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	let status = $state<'loading' | 'success' | 'error'>('loading');
	let message = $state('Confirming your new email address...');

	$effect(() => {
		const token = router.queryParam('token');
		if (!token) {
			status = 'error';
			message = 'Confirmation token is missing.';
			return;
		}
		authApi
			.confirmEmailChange(token)
			.then(async (res) => {
				await authStore.refreshUser();
				status = 'success';
				message = res.message;
			})
			.catch((err) => {
				status = 'error';
				message = err instanceof ApiError ? err.message : 'Confirmation token is invalid or expired.';
			});
	});
</script>

<AuthShell title="Confirm email change">
	<div class="space-y-4">
		<div
			class="rounded-md border-2 border-ink p-4 text-sm font-medium text-ink {status === 'error'
				? 'bg-coral/25'
				: 'bg-yellow/40'}"
		>
			{message}
		</div>
		<Button fullWidth onclick={() => router.navigate(authStore.user ? '/dashboard' : '/login', true)}>
			{authStore.user ? 'Back to Dashboard' : 'Go to Sign In'}
		</Button>
	</div>
</AuthShell>
