<script lang="ts" module>
	import { cva, type VariantProps } from 'class-variance-authority';

	export const badgeVariants = cva(
		'inline-flex items-center gap-1.5 border-2 border-ink font-heading font-bold uppercase tracking-wider select-none rounded-sm px-2.5 py-0.5 text-xs',
		{
			variants: {
				variant: {
					yellow: 'bg-yellow text-ink shadow-brutal-xs',
					cobalt: 'bg-cobalt text-surface shadow-brutal-xs',
					coral: 'bg-coral text-ink shadow-brutal-xs',
					cream: 'bg-cream text-ink shadow-brutal-xs'
				}
			},
			defaultVariants: {
				variant: 'yellow'
			}
		}
	);

	export type BadgeVariant = VariantProps<typeof badgeVariants>['variant'];
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import { cn } from '$lib/utils/cn';

	interface Props {
		variant?: BadgeVariant;
		class?: string;
		children?: Snippet;
	}

	let { variant = 'yellow', class: className = '', children }: Props = $props();
</script>

<span class={cn(badgeVariants({ variant }), className)}>
	{@render children?.()}
</span>
