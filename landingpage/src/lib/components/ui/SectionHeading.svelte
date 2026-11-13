<script lang="ts">
	import Badge, { type BadgeVariant } from './Badge.svelte';
	import { cn } from '$lib/utils/cn';

	interface Props {
		eyebrow?: string;
		badgeVariant?: BadgeVariant;
		title: string;
		description?: string;
		align?: 'left' | 'center';
		invert?: boolean;
		class?: string;
	}

	let {
		eyebrow,
		badgeVariant = 'yellow',
		title,
		description,
		align = 'center',
		invert = false,
		class: className = ''
	}: Props = $props();
</script>

<div
	class={cn(
		'flex flex-col gap-4',
		align === 'center' ? 'mx-auto max-w-3xl items-center text-center' : 'max-w-2xl items-start text-left',
		className
	)}
>
	{#if eyebrow}
		<Badge variant={badgeVariant} size="md" shadow="xs">
			<span class="inline-block h-2 w-2 rounded-none bg-current"></span>
			{eyebrow}
		</Badge>
	{/if}

	<h2
		class={cn(
			'font-heading text-3xl font-extrabold tracking-tight sm:text-4xl lg:text-5xl leading-[1.08]',
			invert ? 'text-surface' : 'text-ink'
		)}
	>
		{title}
	</h2>

	{#if description}
		<p
			class={cn(
				'text-base sm:text-lg leading-relaxed font-normal',
				invert ? 'text-cream' : 'text-ink/80'
			)}
		>
			{description}
		</p>
	{/if}
</div>
