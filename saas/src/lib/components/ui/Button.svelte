<script lang="ts" module>
	import { cva, type VariantProps } from 'class-variance-authority';

	export const buttonVariants = cva(
		'inline-flex items-center justify-center gap-2 font-heading font-bold tracking-tight border-2 border-ink rounded-md transition-all duration-150 cursor-pointer select-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt focus-visible:ring-offset-2 focus-visible:ring-offset-cream disabled:opacity-50 disabled:pointer-events-none',
		{
			variants: {
				variant: {
					primary:
						'bg-cobalt text-surface shadow-brutal hover:-translate-x-0.5 hover:-translate-y-0.5 hover:shadow-brutal-md active:translate-x-0.5 active:translate-y-0.5 active:shadow-brutal-xs',
					yellow:
						'bg-yellow text-ink shadow-brutal hover:-translate-x-0.5 hover:-translate-y-0.5 hover:shadow-brutal-md active:translate-x-0.5 active:translate-y-0.5 active:shadow-brutal-xs',
					coral:
						'bg-coral text-ink shadow-brutal hover:-translate-x-0.5 hover:-translate-y-0.5 hover:shadow-brutal-md active:translate-x-0.5 active:translate-y-0.5 active:shadow-brutal-xs',
					secondary:
						'bg-surface text-ink shadow-brutal hover:bg-yellow hover:-translate-x-0.5 hover:-translate-y-0.5 hover:shadow-brutal-md active:translate-x-0.5 active:translate-y-0.5 active:shadow-brutal-xs',
					ghost:
						'border-transparent bg-transparent text-ink shadow-none hover:border-ink hover:bg-surface hover:shadow-brutal-xs'
				},
				size: {
					sm: 'h-9 px-3.5 text-xs uppercase tracking-wider',
					md: 'h-11 px-5 text-sm',
					lg: 'h-12 px-6 text-base'
				},
				fullWidth: {
					true: 'w-full',
					false: ''
				}
			},
			defaultVariants: {
				variant: 'primary',
				size: 'md',
				fullWidth: false
			}
		}
	);

	export type ButtonVariant = VariantProps<typeof buttonVariants>['variant'];
	export type ButtonSize = VariantProps<typeof buttonVariants>['size'];
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLAnchorAttributes, HTMLButtonAttributes } from 'svelte/elements';
	import { cn } from '$lib/utils/cn';

	interface BaseProps {
		variant?: ButtonVariant;
		size?: ButtonSize;
		fullWidth?: boolean;
		class?: string;
		children?: Snippet;
	}

	type ButtonElementProps = BaseProps & HTMLButtonAttributes & { href?: undefined };
	type AnchorElementProps = BaseProps & HTMLAnchorAttributes & { href: string };
	type Props = ButtonElementProps | AnchorElementProps;

	let {
		variant = 'primary',
		size = 'md',
		fullWidth = false,
		class: className = '',
		href,
		children,
		...restProps
	}: Props = $props();
</script>

{#if href}
	<a
		{href}
		class={cn(buttonVariants({ variant, size, fullWidth }), className)}
		{...(restProps as HTMLAnchorAttributes)}
	>
		{@render children?.()}
	</a>
{:else}
	<button
		type="button"
		class={cn(buttonVariants({ variant, size, fullWidth }), className)}
		{...(restProps as HTMLButtonAttributes)}
	>
		{@render children?.()}
	</button>
{/if}
