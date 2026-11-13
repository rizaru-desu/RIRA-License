<script lang="ts">
	import {
		KeyRound,
		CheckCircle2,
		Cpu,
		SlidersHorizontal,
		ShieldCheck,
		Webhook,
		Check,
		Info
	} from '@lucide/svelte';
	import SectionHeading from '$lib/components/ui/SectionHeading.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { coreFeaturesList, type CoreFeatureItem } from '$lib/data/features';

	function getIconBoxClasses(accent: CoreFeatureItem['accent']): string {
		switch (accent) {
			case 'yellow':
				return 'bg-yellow text-ink';
			case 'cobalt':
				return 'bg-cobalt text-surface';
			case 'coral':
				return 'bg-coral text-ink';
			case 'white':
				return 'bg-cream text-ink';
		}
	}

	function getStatusBadgeVariant(
		status: CoreFeatureItem['statusLabel']
	): 'yellow' | 'cobalt' | 'cream' {
		switch (status) {
			case 'AVAILABLE FOUNDATION':
				return 'yellow';
			case 'IN DEVELOPMENT (v1.0)':
				return 'cobalt';
			case 'PLANNED (v1.0)':
				return 'cream';
		}
	}
</script>

<section id="features" class="border-b-2 border-ink bg-cream py-16 sm:py-24">
	<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
		<SectionHeading
			eyebrow="CORE FEATURES"
			badgeVariant="yellow"
			title="Complete Licensing Primitives. Zero Artificial Limits."
			description="RiraLicense Community Edition is designed as a full-featured, self-hosted licensing platform—covering key lifecycle management, device activation, feature entitlements, and offline digital signatures."
		/>

		<div class="mt-14 grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3 lg:gap-8">
			{#each coreFeaturesList as feature (feature.id)}
				<Card
					variant="white"
					shadow="md"
					padding="lg"
					interactive={true}
					class="flex flex-col justify-between"
				>
					<div>
						<!-- Top Icon & Status Badge -->
						<div class="flex flex-wrap items-center justify-between gap-3">
							<div
								class="flex h-12 w-12 items-center justify-center rounded-md border-2 border-ink shadow-brutal-xs {getIconBoxClasses(feature.accent)}"
							>
								{#if feature.iconName === 'key-round'}
									<KeyRound class="h-6 w-6 stroke-[2.25]" />
								{:else if feature.iconName === 'check-circle-2'}
									<CheckCircle2 class="h-6 w-6 stroke-[2.25]" />
								{:else if feature.iconName === 'cpu'}
									<Cpu class="h-6 w-6 stroke-[2.25]" />
								{:else if feature.iconName === 'sliders-horizontal'}
									<SlidersHorizontal class="h-6 w-6 stroke-[2.25]" />
								{:else if feature.iconName === 'shield-check'}
									<ShieldCheck class="h-6 w-6 stroke-[2.25]" />
								{:else}
									<Webhook class="h-6 w-6 stroke-[2.25]" />
								{/if}
							</div>

							<div class="flex flex-col items-end gap-1">
								<span class="font-mono text-[10px] font-bold uppercase tracking-wider text-ink/60">
									{feature.number} • {feature.tag}
								</span>
								<Badge
									variant={getStatusBadgeVariant(feature.statusLabel)}
									size="sm"
									shadow="none"
								>
									{feature.statusLabel}
								</Badge>
							</div>
						</div>

						<!-- Title & Description -->
						<h3 class="mt-6 font-heading text-xl font-extrabold text-ink sm:text-2xl">
							{feature.title}
						</h3>

						<p class="mt-2.5 text-sm sm:text-base leading-relaxed text-ink/80">
							{feature.description}
						</p>
					</div>

					<!-- Highlights Checklist -->
					<ul class="mt-6 space-y-2 border-t-2 border-ink/15 pt-4">
						{#each feature.highlights as highlight (highlight)}
							<li class="flex items-start gap-2.5 text-xs sm:text-sm font-semibold text-ink">
								<span
									class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-none border border-ink bg-yellow text-ink"
								>
									<Check class="h-3 w-3 stroke-[3]" />
								</span>
								<span>{highlight}</span>
							</li>
						{/each}
					</ul>
				</Card>
			{/each}
		</div>

		<!-- Transparent Implementation Status Callout -->
		<div
			class="mt-10 flex flex-col items-start justify-between gap-4 rounded-md border-2 border-ink bg-surface p-5 shadow-brutal sm:flex-row sm:items-center sm:px-6"
		>
			<div class="flex items-start gap-3 sm:items-center">
				<div
					class="flex h-9 w-9 shrink-0 items-center justify-center rounded-sm border-2 border-ink bg-yellow text-ink shadow-brutal-xs"
				>
					<Info class="h-4 w-4 stroke-[2.5]" />
				</div>
				<p class="text-xs sm:text-sm font-medium leading-relaxed text-ink/85">
					<strong class="font-heading font-extrabold text-ink">
						Transparent Open-Source Status:
					</strong>
					The Go + chi HTTP server foundation and Svelte 5 web interface are available today. Core PostgreSQL
					licensing endpoints, offline Ed25519 signature verification, and official SDKs are actively
					being built for the
					<a href="#community" class="font-bold text-cobalt underline underline-offset-4 hover:text-ink">
						Community Edition v1.0 milestone
					</a>.
				</p>
			</div>
		</div>
	</div>
</section>
