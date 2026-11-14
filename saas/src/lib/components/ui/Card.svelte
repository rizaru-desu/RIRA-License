<script lang="ts" module>
	import { cva, type VariantProps } from 'class-variance-authority';

	export const cardVariants = cva('border-2 border-ink rounded-md transition-all duration-150', {
		variants: {
			variant: {
				white: 'bg-surface text-ink',
				cream: 'bg-cream text-ink',
				yellow: 'bg-yellow text-ink',
				coral: 'bg-coral text-ink'
			},
			shadow: {
				none: 'shadow-none',
				sm: 'shadow-brutal-sm',
				md: 'shadow-brutal',
				lg: 'shadow-brutal-md'
			},
			padding: {
				sm: 'p-4',
				md: 'p-6',
				lg: 'p-6 sm:p-8'
			}
		},
		defaultVariants: {
			variant: 'white',
			shadow: 'md',
			padding: 'md'
		}
	});

	export type CardVariant = VariantProps<typeof cardVariants>['variant'];
	export type CardShadow = VariantProps<typeof cardVariants>['shadow'];
	export type CardPadding = VariantProps<typeof cardVariants>['padding'];
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLAttributes } from 'svelte/elements';
	import { cn } from '$lib/utils/cn';

	interface Props extends HTMLAttributes<HTMLDivElement> {
		variant?: CardVariant;
		shadow?: CardShadow;
		padding?: CardPadding;
		class?: string;
		children?: Snippet;
	}

	let {
		variant = 'white',
		shadow = 'md',
		padding = 'md',
		class: className = '',
		children,
		...restProps
	}: Props = $props();
</script>

<div class={cn(cardVariants({ variant, shadow, padding }), className)} {...restProps}>
	{@render children?.()}
</div>
