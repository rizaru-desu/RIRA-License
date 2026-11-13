<script lang="ts">
	import { KeyRound, Menu, X, ArrowUpRight, ArrowRight, CodeXml } from '@lucide/svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { primaryNavItems, siteConfig } from '$lib/data/navigation';

	let mobileMenuOpen = $state(false);

	function toggleMobileMenu() {
		mobileMenuOpen = !mobileMenuOpen;
	}

	function closeMobileMenu() {
		mobileMenuOpen = false;
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && mobileMenuOpen) {
			mobileMenuOpen = false;
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<header class="sticky top-0 z-50 w-full border-b-2 border-ink bg-cream">
	<div class="mx-auto flex h-20 max-w-7xl items-center justify-between gap-4 px-4 sm:px-6 lg:px-8">
		<!-- Brand Logo -->
		<a
			href="#top"
			onclick={closeMobileMenu}
			class="group inline-flex items-center gap-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt"
			aria-label="RiraLicense Home"
		>
			<div
				class="flex h-11 w-11 items-center justify-center rounded-md border-2 border-ink bg-yellow shadow-brutal-xs transition-transform duration-150 group-hover:-translate-x-0.5 group-hover:-translate-y-0.5"
			>
				<KeyRound class="h-5 w-5 text-ink stroke-[2.5]" />
			</div>
			<div class="flex flex-col">
				<div class="flex items-center gap-2">
					<span class="font-heading text-xl font-extrabold tracking-tight text-ink leading-none">
						Rira<span class="text-cobalt">License</span>
					</span>
					<span
						class="hidden sm:inline-block rounded-none border border-ink bg-surface px-1.5 py-0.5 font-mono text-[9px] font-extrabold uppercase tracking-wider text-ink"
					>
						OSS
					</span>
				</div>
				<span class="mt-0.5 font-mono text-[10px] font-bold uppercase tracking-widest text-ink/70">
					OPEN-SOURCE LICENSING
				</span>
			</div>
		</a>

		<!-- Desktop Navigation: Features, Documentation, API Reference, GitHub -->
		<nav aria-label="Primary Navigation" class="hidden md:flex md:items-center md:gap-1.5">
			{#each primaryNavItems as item (item.label)}
				<a
					href={item.href}
					target={item.external ? '_blank' : undefined}
					rel={item.external ? 'noreferrer' : undefined}
					class="inline-flex items-center gap-1 rounded-md border-2 border-transparent px-3.5 py-2 font-heading text-sm font-bold text-ink transition-all duration-150 hover:border-ink hover:bg-surface hover:shadow-brutal-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt"
				>
					<span>{item.label}</span>
					{#if item.external}
						<ArrowUpRight class="h-3.5 w-3.5 stroke-[2.5] text-cobalt" />
					{/if}
				</a>
			{/each}
		</nav>

		<!-- Right Side: AGPL-3.0 Badge + Get Started CTA -->
		<div class="hidden md:flex md:items-center md:gap-3">
			<a
				href={siteConfig.githubUrl}
				target="_blank"
				rel="noreferrer"
				class="hidden lg:inline-flex h-10 items-center gap-2 rounded-md border-2 border-ink bg-surface px-3 font-mono text-xs font-bold text-ink shadow-brutal-xs transition-all duration-150 hover:bg-yellow focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt"
				aria-label="RiraLicense Open Source Repository on GitHub"
			>
				<CodeXml class="h-4 w-4 stroke-[2.5] text-cobalt" />
				<span>{siteConfig.coreLicense}</span>
			</a>

			<Button
				variant="primary"
				size="md"
				href={siteConfig.getStartedHref}
				onclick={closeMobileMenu}
			>
				Get Started
				<ArrowRight class="h-4 w-4 stroke-[2.5]" />
			</Button>
		</div>

		<!-- Mobile Menu Toggle -->
		<div class="flex items-center gap-2 md:hidden">
			<button
				type="button"
				onclick={toggleMobileMenu}
				aria-expanded={mobileMenuOpen}
				aria-controls="mobile-nav-menu"
				aria-label={mobileMenuOpen ? 'Close navigation menu' : 'Open navigation menu'}
				class="inline-flex h-11 w-11 items-center justify-center rounded-md border-2 border-ink bg-surface text-ink shadow-brutal-xs transition-all active:translate-x-0.5 active:translate-y-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt"
			>
				{#if mobileMenuOpen}
					<X class="h-5 w-5 stroke-[2.5]" />
				{:else}
					<Menu class="h-5 w-5 stroke-[2.5]" />
				{/if}
			</button>
		</div>
	</div>

	<!-- Mobile Navigation Menu -->
	{#if mobileMenuOpen}
		<div
			id="mobile-nav-menu"
			class="border-t-2 border-ink bg-surface px-4 pt-4 pb-6 shadow-brutal md:hidden"
		>
			<nav aria-label="Mobile Navigation" class="flex flex-col gap-2">
				{#each primaryNavItems as item (item.label)}
					<a
						href={item.href}
						target={item.external ? '_blank' : undefined}
						rel={item.external ? 'noreferrer' : undefined}
						onclick={closeMobileMenu}
						class="flex items-center justify-between rounded-md border-2 border-ink bg-cream px-4 py-3 font-heading text-base font-bold text-ink shadow-brutal-xs transition-colors hover:bg-yellow"
					>
						<span>{item.label}</span>
						{#if item.external}
							<ArrowUpRight class="h-4 w-4 stroke-[2.5]" />
						{:else}
							<ArrowRight class="h-4 w-4 stroke-[2.5]" />
						{/if}
					</a>
				{/each}
			</nav>

			<div class="mt-5 border-t-2 border-ink/20 pt-5">
				<Button
					variant="primary"
					size="lg"
					fullWidth
					href={siteConfig.getStartedHref}
					onclick={closeMobileMenu}
				>
					Get Started
					<ArrowRight class="h-4 w-4 stroke-[2.5]" />
				</Button>
			</div>
		</div>
	{/if}
</header>
