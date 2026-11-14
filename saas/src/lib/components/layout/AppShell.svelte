<script lang="ts">
	import type { Snippet } from 'svelte';
	import { KeyRound, LayoutDashboard, ShieldCheck, LogOut } from '@lucide/svelte';
	import { authStore } from '$lib/stores/auth.svelte';
	import { router } from '$lib/stores/router.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	interface Props {
		children?: Snippet;
	}

	let { children }: Props = $props();
	let loggingOut = $state(false);

	async function handleLogout() {
		loggingOut = true;
		try {
			await authStore.logout();
			router.navigate('/login', true);
		} finally {
			loggingOut = false;
		}
	}
</script>

<div class="min-h-screen bg-cream bg-brutal-grid flex flex-col">
	<header class="border-b-2 border-ink bg-surface">
		<div class="mx-auto flex h-16 max-w-6xl items-center justify-between gap-4 px-4 sm:px-6">
			<div class="flex items-center gap-6">
				<button
					type="button"
					onclick={() => router.navigate('/dashboard')}
					class="flex items-center gap-2.5 cursor-pointer"
				>
					<div
						class="flex h-9 w-9 items-center justify-center rounded-md border-2 border-ink bg-yellow shadow-brutal-xs"
					>
						<KeyRound class="h-4 w-4 text-ink stroke-[2.5]" />
					</div>
					<span class="font-heading text-lg font-extrabold tracking-tight text-ink">
						Rira<span class="text-cobalt">License</span>
					</span>
				</button>

				<nav class="flex items-center gap-2">
					<button
						type="button"
						onclick={() => router.navigate('/dashboard')}
						class="inline-flex items-center gap-1.5 rounded-md border-2 px-3 py-1.5 font-heading text-sm font-bold cursor-pointer transition-all {router.pathname ===
						'/dashboard'
							? 'border-ink bg-yellow shadow-brutal-xs'
							: 'border-transparent hover:border-ink hover:bg-cream'}"
					>
						<LayoutDashboard class="h-4 w-4" />
						<span>Overview</span>
					</button>
					<button
						type="button"
						onclick={() => router.navigate('/settings/security')}
						class="inline-flex items-center gap-1.5 rounded-md border-2 px-3 py-1.5 font-heading text-sm font-bold cursor-pointer transition-all {router.pathname ===
						'/settings/security'
							? 'border-ink bg-yellow shadow-brutal-xs'
							: 'border-transparent hover:border-ink hover:bg-cream'}"
					>
						<ShieldCheck class="h-4 w-4" />
						<span>Security</span>
					</button>
				</nav>
			</div>

			<div class="flex items-center gap-3">
				{#if authStore.user}
					<span class="hidden sm:inline-block font-mono text-xs font-bold text-ink/75">
						{authStore.user.email}
					</span>
				{/if}
				<Button
					variant="secondary"
					size="sm"
					disabled={loggingOut}
					onclick={handleLogout}
				>
					<LogOut class="h-3.5 w-3.5" />
					<span>{loggingOut ? 'Signing out...' : 'Logout'}</span>
				</Button>
			</div>
		</div>
	</header>

	<main class="mx-auto w-full max-w-6xl flex-1 px-4 py-8 sm:px-6">
		{@render children?.()}
	</main>
</div>
