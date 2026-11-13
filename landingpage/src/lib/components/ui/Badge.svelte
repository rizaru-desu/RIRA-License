<script lang="ts" module>
	import { cva, type VariantProps } from 'class-variance-authority';

	export const badgeVariants = cva(
		'inline-flex items-center gap-1.5 border-2 border-ink font-heading font-bold uppercase tracking-wider select-none rounded-sm',
		{
			variants: {
				variant: {
					yellow: 'bg-yellow text-ink',
					cobalt: 'bg-cobalt text-surface',
					coral: 'bg-coral text-ink',
					white: 'bg-surface text-ink',
					cream: 'bg-cream text-ink',
					dark: 'bg-ink text-surface'
				},
				size: {
					sm: 'px-2 py-0.5 text-[11px] leading-tight',
					md: 'px-3 py-1 text-xs leading-tight',
					lg: 'px-4 py-1.5 text-xs sm:text-sm leading-tight'
				},
				shadow: {
					none: 'shadow-none',
					xs: 'shadow-brutal-xs',
					sm: 'shadow-brutal-sm'
				}
			},
			defaultVariants: {
				variant: 'yellow',
				size: 'md',
				shadow: 'xs'
			}
		}
	);

	export type BadgeVariant = VariantProps<typeof badgeVariants>['variant'];
	export type BadgeSize = VariantProps<typeof badgeVariants>['size'];
	export type BadgeShadow = VariantProps<typeof badgeVariants>['shadow'];
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLAttributes } from 'svelte/elements';
	import { cn } from '$lib/utils/cn';

	interface Props extends HTMLAttributes<HTMLSpanElement> {
		variant?: BadgeVariant;
		size?: BadgeSize;
		shadow?: BadgeShadow;
		class?: string;
		children?: Snippet;
	}

	let {
		variant = 'yellow',
		size = 'md',
		shadow = 'xs',
		class: className = '',
		children,
		...restProps
	}: Props = $props();
</script>

<span class={cn(badgeVariants({ variant, size, shadow }), className)} {...restProps}>
	{@render children?.()}
</span>
