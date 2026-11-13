<script lang="ts">
	import { ArrowRight, ArrowUpRight, BookOpen, KeyRound, CheckCircle2 } from '@lucide/svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { siteConfig } from '$lib/data/navigation';
</script>

<section id="final-cta" class="bg-cream py-16 sm:py-24">
	<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
		<div
			class="relative overflow-hidden rounded-md border-2 border-ink bg-yellow bg-brutal-grid p-8 text-ink shadow-brutal-lg sm:p-12 lg:p-16"
		>
			<div class="grid grid-cols-1 items-center gap-10 lg:grid-cols-12">
				<!-- Left Copy -->
				<div class="lg:col-span-7">
					<Badge variant="dark" size="md" shadow="xs" class="mb-5">
						<KeyRound class="h-3.5 w-3.5 text-yellow stroke-[2.5]" />
						OPEN SOURCE • SELF HOSTED • DEVELOPER FIRST
					</Badge>

					<h2
						class="font-heading text-3xl font-extrabold tracking-tight text-ink sm:text-4xl lg:text-5xl leading-[1.06]"
					>
						Start Building with RiraLicense.
					</h2>

					<p class="mt-5 max-w-2xl text-base sm:text-lg font-medium leading-relaxed text-ink/85">
						Deploy open-source software licensing infrastructure on your own servers. Create,
						manage, validate, and protect software licenses across desktop applications, SaaS
						products, and APIs.
					</p>

					<div class="mt-6 flex flex-wrap items-center gap-3 font-mono text-xs font-bold text-ink">
						<span
							class="inline-flex items-center gap-1.5 rounded-sm border-2 border-ink bg-surface px-3 py-1 shadow-brutal-xs"
						>
							<CheckCircle2 class="h-3.5 w-3.5 text-cobalt stroke-[2.5]" />
							GNU AGPL-3.0 LICENSE
						</span>
						<span
							class="inline-flex items-center + gap-1.5 rounded-sm border-2 border-ink bg-surface px-3 py-1 shadow-brutal-xs"
						>
							<CheckCircle2 class="h-3.5 w-3.5 text-cobalt stroke-[2.5]" />
							DOCKER COMPOSE &amp; POSTGRESQL
						</span>
						<span
							class="inline-flex items-center gap-1.5 rounded-sm border-2 border-ink bg-surface px-3 py-1 shadow-brutal-xs"
						>
							<CheckCircle2 class="h-3.5 w-3.5 text-cobalt stroke-[2.5]" />
							ZERO EXTERNAL LOCK-IN
						</span>
					</div>
				</div>

				<!-- Right CTAs Box: Get Started, View Documentation, Explore GitHub -->
				<div class="lg:col-span-5">
					<div
						class="flex flex-col gap-3.5 rounded-md border-2 border-ink bg-surface p-6 text-ink shadow-brutal-md"
					>
						<div class="font-mono text-xs font-extrabold uppercase tracking-wider text-cobalt">
							RIRALICENSE COMMUNITY EDITION
						</div>

						<Button
							variant="primary"
							size="xl"
							fullWidth={true}
							href={siteConfig.getStartedHref}
						>
							Get Started
							<ArrowRight class="h-5 w-5 stroke-[2.5]" />
						</Button>

						<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
							<Button
								variant="secondary"
								size="lg"
								fullWidth={true}
								href={siteConfig.apiReferenceHref}
							>
								<BookOpen class="h-4 w-4 stroke-[2.25]" />
								View Documentation
							</Button>

							<Button
								variant="cream"
								size="lg"
								fullWidth={true}
								href={siteConfig.githubUrl}
								target="_blank"
								rel="noreferrer"
							>
								Explore GitHub
								<ArrowUpRight class="h-4 w-4 stroke-[2.5]" />
							</Button>
						</div>

						<p class="text-center font-mono text-[11px] font-semibold text-ink/70">
							100% Open Source • Self-Hosted Infrastructure
						</p>
					</div>
				</div>
			</div>
		</div>
	</div>
</section>
