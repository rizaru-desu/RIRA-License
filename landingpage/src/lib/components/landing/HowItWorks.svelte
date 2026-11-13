<script lang="ts">
	import { ArrowRight } from '@lucide/svelte';
	import SectionHeading from '$lib/components/ui/SectionHeading.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import { howItWorksSteps } from '$lib/data/features';

	function getStepBoxClasses(accent: 'yellow' | 'cobalt' | 'coral' | 'white'): string {
		switch (accent) {
			case 'yellow':
				return 'bg-yellow text-ink';
			case 'cobalt':
				return 'bg-cobalt text-surface';
			case 'coral':
				return 'bg-coral text-ink';
			case 'white':
				return 'bg-surface text-ink';
		}
	}
</script>

<section id="how-it-works" class="border-b-2 border-ink bg-surface py-16 sm:py-24">
	<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
		<SectionHeading
			eyebrow="HOW IT WORKS"
			badgeVariant="coral"
			title="From Product Setup to Validated Licenses."
			description="Integrate RiraLicense Community Edition into your software workflow in four structured steps."
		/>

		<div class="relative mt-14 grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4">
			{#each howItWorksSteps as item, idx (item.step)}
				<Card
					variant="cream"
					shadow="lg"
					padding="none"
					interactive={true}
					class="flex flex-col justify-between overflow-hidden"
				>
					<div>
						<div
							class="flex items-center justify-between border-b-2 border-ink px-6 py-5 {getStepBoxClasses(item.accent)}"
						>
							<span class="font-heading text-4xl font-extrabold tracking-tight sm:text-5xl leading-none">
								{item.step}
							</span>
							<div class="flex items-center gap-1.5">
								<span
									class="rounded-none border border-ink bg-surface px-2 py-0.5 font-mono text-[10px] font-extrabold uppercase tracking-wider text-ink"
								>
									STEP {item.step}
								</span>
								{#if idx < howItWorksSteps.length - 1}
									<ArrowRight class="hidden h-4 w-4 stroke-[2.75] lg:inline" />
								{/if}
							</div>
						</div>

						<div class="p-6">
							<div class="font-mono text-[11px] font-bold uppercase tracking-wider text-cobalt">
								{item.subtitle}
							</div>
							<h3 class="mt-1.5 font-heading text-xl font-extrabold text-ink leading-snug">
								{item.title}
							</h3>
							<p class="mt-3 text-sm leading-relaxed text-ink/80">
								{item.description}
							</p>
						</div>
					</div>

					<div class="border-t-2 border-ink bg-surface px-6 py-3 font-mono text-[11px] font-bold text-ink">
						{item.deliverable}
					</div>
				</Card>
			{/each}
		</div>
	</div>
</section>
