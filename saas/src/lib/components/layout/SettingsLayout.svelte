<script lang="ts">
	import type { Snippet } from 'svelte';
	import { User, ShieldCheck, ExternalLink } from '@lucide/svelte';
	import { router } from '$lib/stores/router.svelte';
	import AppShell from '$lib/components/layout/AppShell.svelte';

	const DOCS_URL = 'https://github.com/rizaru-desu/RIRA-License#readme';

	interface Props {
		children?: Snippet;
	}

	let { children }: Props = $props();

	const tabs = [
		{
			id: 'profile',
			label: 'Profile',
			href: '/settings/profile',
			icon: User
		},
		{
			id: 'security',
			label: 'Security',
			href: '/settings/security',
			icon: ShieldCheck
		}
	];
</script>

<AppShell>
	<div class="space-y-6">
		<div
			role="tablist"
			aria-label="Settings tabs"
			class="inline-flex flex-wrap gap-2 rounded-md border-2 border-ink bg-surface p-1.5 shadow-brutal-xs"
		>
			{#each tabs as tab (tab.href)}
				{@const active = router.pathname === tab.href}
				{@const Icon = tab.icon}
				<button
					type="button"
					role="tab"
					aria-selected={active}
					onclick={() => router.navigate(tab.href)}
					class="inline-flex items-center gap-2 rounded border-2 px-4 py-2 font-heading text-sm font-extrabold cursor-pointer transition-all {active
						? 'border-ink bg-yellow text-ink shadow-brutal-xs'
						: 'border-transparent bg-transparent text-ink/75 hover:border-ink hover:bg-cream hover:text-ink'}"
				>
					<Icon class="h-4 w-4 shrink-0" aria-hidden="true" />
					<span>{tab.label}</span>
				</button>
			{/each}
		</div>

		{@render children?.()}

		<div class="border-t-2 border-ink/15 pt-4">
			<a
				href={DOCS_URL}
				target="_blank"
				rel="noopener noreferrer"
				class="inline-flex items-center gap-1.5 text-xs font-medium text-ink/75 transition-colors hover:text-ink"
			>
				<span>
					Need help?
					<span class="font-heading font-bold text-cobalt underline">View documentation</span>
				</span>
				<ExternalLink class="h-3.5 w-3.5 shrink-0 text-cobalt" aria-hidden="true" />
			</a>
		</div>
	</div>
</AppShell>

