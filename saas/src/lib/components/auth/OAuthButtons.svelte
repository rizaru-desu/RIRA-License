<script lang="ts">
	import { authApi } from '$lib/api/auth';
	import { authStore } from '$lib/stores/auth.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	const providers = [
		{ id: 'google', label: 'Continue with Google' },
		{ id: 'github', label: 'Continue with GitHub' }
	] as const;

	let oauthNotice = $state('');

	function handleProviderClick(provider: (typeof providers)[number]['id']) {
		oauthNotice = '';
		if (authStore.providers.includes(provider)) {
			window.location.href = authApi.oauthStartUrl(provider);
			return;
		}
		const envPrefix = provider.toUpperCase();
		oauthNotice = `${provider === 'google' ? 'Google' : 'GitHub'} OAuth is not configured on the server yet (set ${envPrefix}_CLIENT_ID and ${envPrefix}_CLIENT_SECRET).`;
	}
</script>

<div class="my-5 flex items-center gap-3">
	<div class="h-0.5 flex-1 bg-ink/20"></div>
	<span class="font-mono text-xs font-bold uppercase tracking-wider text-ink/60">
		or continue with
	</span>
	<div class="h-0.5 flex-1 bg-ink/20"></div>
</div>

{#if oauthNotice}
	<div
		role="alert"
		class="mb-3 rounded-md border-2 border-ink bg-yellow/40 px-3 py-2 text-xs font-semibold text-ink"
	>
		{oauthNotice}
	</div>
{/if}

<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
	{#each providers as p (p.id)}
		<Button
			variant="secondary"
			fullWidth
			onclick={() => handleProviderClick(p.id)}
			data-provider={p.id}
		>
			{#if p.id === 'github'}
				<svg class="h-4 w-4 shrink-0" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
					<path
						d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z"
					/>
				</svg>
				<span>GitHub</span>
			{:else if p.id === 'google'}
				<svg class="h-4 w-4 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
					<path
						fill="currentColor"
						d="M21.35 11.1h-9.17v2.73h6.51c-.33 3.81-3.5 5.44-6.5 5.44C8.36 19.27 5 16.25 5 12c0-4.1 3.2-7.27 7.2-7.27 3.09 0 4.9 1.97 4.9 1.97L19 4.72S16.56 2 12.1 2C6.42 2 2.03 6.8 2.03 12c0 5.05 4.13 10 10.22 10 5.35 0 9.25-3.67 9.25-9.09 0-1.15-.15-1.81-.15-1.81Z"
					/>
				</svg>
				<span>Google</span>
			{/if}
		</Button>
	{/each}
</div>
