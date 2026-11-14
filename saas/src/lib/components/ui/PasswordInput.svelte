<script lang="ts">
	import { Eye, EyeOff } from '@lucide/svelte';
	import { cn } from '$lib/utils/cn';

	interface Props {
		id?: string;
		value?: string;
		placeholder?: string;
		autocomplete?: AutoFill;
		required?: boolean;
		minlength?: number;
		maxlength?: number;
		disabled?: boolean;
		class?: string;
	}

	let {
		id,
		value = $bindable(''),
		placeholder = '••••••••••••',
		autocomplete,
		required = false,
		minlength,
		maxlength,
		disabled = false,
		class: className = ''
	}: Props = $props();

	let visible = $state(false);
</script>

<div class="relative w-full">
	<input
		{id}
		type={visible ? 'text' : 'password'}
		{autocomplete}
		{required}
		{minlength}
		{maxlength}
		{disabled}
		{placeholder}
		bind:value
		class={cn(
			'w-full rounded-md border-2 border-ink bg-cream py-2.5 pr-11 pl-3.5 text-sm text-ink focus:bg-surface focus:outline-none focus:ring-2 focus:ring-cobalt disabled:opacity-50',
			className
		)}
	/>
	<button
		type="button"
		tabindex={0}
		onclick={() => (visible = !visible)}
		aria-label={visible ? 'Hide password' : 'Show password'}
		class="absolute inset-y-0 right-0 flex items-center justify-center px-3 text-ink/70 hover:text-ink focus-visible:outline-none focus-visible:text-cobalt cursor-pointer"
	>
		{#if visible}
			<EyeOff class="h-4 w-4 stroke-[2.25]" />
		{:else}
			<Eye class="h-4 w-4 stroke-[2.25]" />
		{/if}
	</button>
</div>

