<script lang="ts">
	import type { Snippet } from 'svelte';
	import '@fontsource-variable/space-grotesk';
	import '@fontsource-variable/inter';
	import '../app.css';

	interface Props {
		children?: Snippet;
	}

	let { children }: Props = $props();
</script>

<div class="min-h-screen flex flex-col bg-cream text-ink font-sans">
	{@render children?.()}
</div>
