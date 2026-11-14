<script lang="ts">
	import type { Snippet } from 'svelte';
	import { KeyRound } from '@lucide/svelte';
	import Card from '$lib/components/ui/Card.svelte';

	interface Props {
		title: string;
		subtitle?: string;
		children?: Snippet;
	}

	let { title, subtitle, children }: Props = $props();
</script>

<div class="min-h-screen bg-cream bg-brutal-grid flex flex-col items-center justify-center px-4 py-10">
	<div class="mb-6 flex items-center gap-3">
		<div
			class="flex h-11 w-11 items-center justify-center rounded-md border-2 border-ink bg-yellow shadow-brutal-xs"
		>
			<KeyRound class="h-5 w-5 text-ink stroke-[2.5]" />
		</div>
		<div class="flex flex-col">
			<span class="font-heading text-2xl font-extrabold tracking-tight text-ink leading-none">
				Rira<span class="text-cobalt">License</span>
			</span>
			<span class="mt-0.5 font-mono text-[10px] font-bold uppercase tracking-widest text-ink/70">
				CONTROL PLANE
			</span>
		</div>
	</div>

	<Card class="w-full max-w-md" padding="lg" shadow="lg">
		<div class="mb-6">
			<h1 class="font-heading text-2xl font-extrabold text-ink">{title}</h1>
			{#if subtitle}
				<p class="mt-1 text-sm text-ink/75">{subtitle}</p>
			{/if}
		</div>

		{@render children?.()}
	</Card>
</div>
