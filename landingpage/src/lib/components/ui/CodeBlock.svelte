<script lang="ts">
	import { Copy, Check, Terminal } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import Badge, { type BadgeVariant } from './Badge.svelte';
	import { cn } from '$lib/utils/cn';

	interface Props {
		title: string;
		language?: string;
		badgeText?: string;
		badgeVariant?: BadgeVariant;
		code: string;
		class?: string;
	}

	let {
		title,
		language = 'bash',
		badgeText,
		badgeVariant = 'yellow',
		code,
		class: className = ''
	}: Props = $props();

	let copied = $state(false);

	function copyCode() {
		if (typeof navigator !== 'undefined' && navigator.clipboard) {
			navigator.clipboard.writeText(code).catch(() => {});
		}
		copied = true;
		toast.success('Snippet Copied', {
			description: `${title} copied to clipboard.`
		});
		setTimeout(() => {
			copied = false;
		}, 2000);
	}
</script>

<div
	class={cn(
		'flex flex-col overflow-hidden rounded-md border-2 border-ink bg-ink text-cream shadow-brutal',
		className
	)}
>
	<!-- Terminal Header Bar -->
	<div
		class="flex flex-wrap items-center justify-between gap-2 border-b-2 border-surface/20 bg-ink px-4 py-3"
	>
		<div class="flex items-center gap-2.5 min-w-0">
			<div class="flex items-center gap-1.5 shrink-0" aria-hidden="true">
				<span class="h-3 w-3 border border-surface/60 bg-coral"></span>
				<span class="h-3 w-3 border border-surface/60 bg-yellow"></span>
				<span class="h-3 w-3 border border-surface/60 bg-cobalt"></span>
			</div>
			<span class="inline-flex items-center gap-1.5 truncate font-mono text-xs font-bold text-cream">
				<Terminal class="h-3.5 w-3.5 text-yellow shrink-0" />
				{title}
			</span>
		</div>

		<div class="flex items-center gap-2 shrink-0">
			{#if badgeText}
				<Badge variant={badgeVariant} size="sm" shadow="none">
					{badgeText}
				</Badge>
			{:else if language}
				<span
					class="rounded-sm border border-surface/30 bg-surface/10 px-2 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wider text-cream/80"
				>
					{language}
				</span>
			{/if}

			<button
				type="button"
				onclick={copyCode}
				class="inline-flex items-center gap-1.5 rounded-sm border border-surface/40 bg-surface/10 px-2.5 py-1 font-mono text-[11px] font-bold text-cream transition-colors hover:bg-yellow hover:text-ink hover:border-ink cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-yellow"
				aria-label="Copy code snippet for {title}"
			>
				{#if copied}
					<Check class="h-3.5 w-3.5 text-yellow stroke-[2.5]" />
					<span>Copied</span>
				{:else}
					<Copy class="h-3.5 w-3.5 stroke-[2.25]" />
					<span>Copy</span>
				{/if}
			</button>
		</div>
	</div>

	<!-- Code Body -->
	<div class="flex-1 overflow-x-auto p-4 sm:p-5">
		<pre class="font-mono text-xs sm:text-[13px] leading-relaxed text-cream"><code>{code}</code></pre>
	</div>
</div>
