<script lang="ts">
	import { KeyRound, ShieldCheck, Terminal, ArrowUpRight } from '@lucide/svelte';
	import { footerNavigation, siteConfig } from '$lib/data/navigation';
</script>

<footer class="border-t-2 border-ink bg-ink text-surface">
	<!-- Top Infrastructure Banner -->
	<div class="border-b-2 border-surface/15 bg-ink">
		<div
			class="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4 px-4 py-3.5 sm:px-6 lg:px-8"
		>
			<div class="flex items-center gap-2.5">
				<span class="inline-block h-3 w-3 border border-ink bg-yellow"></span>
				<span class="font-mono text-xs font-bold uppercase tracking-wider text-cream">
					OPEN SOURCE • SELF HOSTED • DEVELOPER FIRST ({siteConfig.coreLicense})
				</span>
			</div>
			<div class="flex items-center gap-4 font-mono text-xs text-cream/80">
				<span class="inline-flex items-center gap-1.5">
					<ShieldCheck class="h-4 w-4 text-yellow" />
					MODULAR MONOLITH ENGINE
				</span>
				<span class="hidden sm:inline text-surface/30">|</span>
				<span class="hidden sm:inline-flex items-center gap-1.5">
					<Terminal class="h-4 w-4 text-coral" />
					GO + CHI • SVELTE 5 • POSTGRESQL
				</span>
			</div>
		</div>
	</div>

	<!-- Main Footer Grid -->
	<div class="mx-auto max-w-7xl px-4 py-14 sm:px-6 lg:px-8 lg:py-16">
		<div class="grid grid-cols-1 gap-10 md:grid-cols-2 lg:grid-cols-12 lg:gap-8">
			<!-- Brand Column (4 cols) -->
			<div class="lg:col-span-4 flex flex-col justify-between space-y-6 pr-0 lg:pr-6">
				<div class="space-y-4">
					<a
						href="#top"
						class="inline-flex items-center gap-3 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-yellow"
					>
						<div
							class="flex h-11 w-11 items-center justify-center rounded-md border-2 border-surface bg-yellow text-ink shadow-brutal-white"
						>
							<KeyRound class="h-5 w-5 stroke-[2.5]" />
						</div>
						<span class="font-heading text-2xl font-extrabold tracking-tight text-surface">
							Rira<span class="text-yellow">License</span>
						</span>
					</a>

					<p class="max-w-sm text-sm sm:text-base leading-relaxed text-cream/85">
						{siteConfig.positioning} {siteConfig.supportingMessage}
					</p>
				</div>

				<!-- Self-Host Clone Command -->
				<div
					class="inline-flex max-w-sm flex-col gap-1.5 rounded-md border-2 border-surface/30 bg-surface/5 p-3.5"
				>
					<span class="font-mono text-[10px] font-bold uppercase tracking-widest text-yellow">
						COMMUNITY EDITION • {siteConfig.coreLicense}
					</span>
					<code class="font-mono text-xs text-cream">
						git clone {siteConfig.githubUrl}.git
					</code>
				</div>
			</div>

			<!-- Documentation Column (2 cols) -->
			<div class="lg:col-span-2 space-y-4">
				<h3
					class="inline-block border-b-2 border-yellow pb-1 font-heading text-xs font-extrabold uppercase tracking-widest text-yellow"
				>
					Documentation
				</h3>
				<ul class="space-y-2.5">
					{#each footerNavigation.documentation as link (link.label)}
						<li>
							<a
								href={link.href}
								class="inline-flex items-center gap-1 text-sm font-medium text-cream/80 transition-colors hover:text-yellow focus-visible:outline-none focus-visible:text-yellow"
							>
								{link.label}
							</a>
						</li>
					{/each}
				</ul>
			</div>

			<!-- API Reference Column (2 cols) -->
			<div class="lg:col-span-2 space-y-4">
				<h3
					class="inline-block border-b-2 border-cobalt pb-1 font-heading text-xs font-extrabold uppercase tracking-widest text-surface"
				>
					API Reference
				</h3>
				<ul class="space-y-2.5">
					{#each footerNavigation.apiReference as link (link.label)}
						<li>
							<a
								href={link.href}
								class="inline-flex items-center gap-1 text-sm font-medium text-cream/80 transition-colors hover:text-yellow focus-visible:outline-none focus-visible:text-yellow"
							>
								{link.label}
							</a>
						</li>
					{/each}
				</ul>
			</div>

			<!-- Community Resources Column (2 cols) -->
			<div class="lg:col-span-2 space-y-4">
				<h3
					class="inline-block border-b-2 border-coral pb-1 font-heading text-xs font-extrabold uppercase tracking-widest text-coral"
				>
					Community
				</h3>
				<ul class="space-y-2.5">
					{#each footerNavigation.community as link (link.label)}
						<li>
							<a
								href={link.href}
								target={'external' in link && link.external ? '_blank' : undefined}
								rel={'external' in link && link.external ? 'noreferrer' : undefined}
								class="inline-flex items-center gap-1 text-sm font-medium text-cream/80 transition-colors hover:text-yellow focus-visible:outline-none focus-visible:text-yellow"
							>
								<span>{link.label}</span>
								{#if 'external' in link && link.external}
									<ArrowUpRight class="h-3.5 w-3.5 opacity-70" />
								{/if}
							</a>
						</li>
					{/each}
				</ul>
			</div>

			<!-- License Information Column (2 cols) -->
			<div class="lg:col-span-2 space-y-4">
				<h3
					class="inline-block border-b-2 border-surface/50 pb-1 font-heading text-xs font-extrabold uppercase tracking-widest text-cream"
				>
					License Info
				</h3>
				<ul class="space-y-2.5">
					{#each footerNavigation.licenseInfo as link (link.label)}
						<li>
							<a
								href={link.href}
								target="_blank"
								rel="noreferrer"
								class="inline-flex items-center gap-1 text-sm font-medium text-cream/80 transition-colors hover:text-yellow focus-visible:outline-none focus-visible:text-yellow"
							>
								<span>{link.label}</span>
								<ArrowUpRight class="h-3.5 w-3.5 opacity-70" />
							</a>
						</li>
					{/each}
				</ul>
			</div>
		</div>

		<!-- Bottom Copyright & License Attribution -->
		<div
			class="mt-12 flex flex-col items-start justify-between gap-4 border-t-2 border-surface/20 pt-8 sm:flex-row sm:items-center"
		>
			<p class="font-mono text-xs sm:text-sm text-cream/85">
				&copy; 2026 RiraLicense (RiraLab — Rizaru Desu). Released under {siteConfig.coreLicense}.
			</p>

			<div class="flex flex-wrap items-center gap-4 font-mono text-xs text-cream/75">
				<a
					href={siteConfig.githubUrl}
					target="_blank"
					rel="noreferrer"
					class="font-semibold text-cream hover:text-yellow"
				>
					GitHub
				</a>
				<span>•</span>
				<a
					href={siteConfig.githubLicenseUrl}
					target="_blank"
					rel="noreferrer"
					class="font-semibold text-yellow underline decoration-yellow decoration-2 underline-offset-4 hover:text-cream"
				>
					{siteConfig.coreLicense}
				</a>
			</div>
		</div>
	</div>
</footer>
