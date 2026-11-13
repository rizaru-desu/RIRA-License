<script lang="ts">
	import {
		CodeXml,
		GitPullRequest,
		Bug,
		MessagesSquare,
		Map,
		ArrowUpRight,
		CheckCircle2,
		Clock
	} from '@lucide/svelte';
	import SectionHeading from '$lib/components/ui/SectionHeading.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import {
		communityCards,
		communityRoadmap,
		siteConfig,
		type CommunityCardItem,
		type RoadmapMilestoneItem
	} from '$lib/data/navigation';

	function getAccentClasses(accent: CommunityCardItem['accent']): string {
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

	function getMilestoneStatusBadge(
		status: RoadmapMilestoneItem['status']
	): 'yellow' | 'cobalt' | 'coral' {
		switch (status) {
			case 'AVAILABLE':
				return 'yellow';
			case 'IN PROGRESS':
				return 'cobalt';
			case 'PLANNED v1.0':
				return 'coral';
		}
	}
</script>

<section id="community" class="border-b-2 border-ink bg-cream py-16 sm:py-24">
	<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
		<SectionHeading
			eyebrow="OPEN SOURCE COMMUNITY"
			badgeVariant="cobalt"
			title="Open Source. Built in the Open."
			description="RiraLicense is developed transparently on GitHub under the GNU AGPL-3.0 license. Inspect the code, report issues, propose improvements, and follow the Community Edition v1.0 roadmap."
		/>

		<!-- 4 Community Pillars: GitHub Repository, Contributions, Issue Reporting, Discussions -->
		<div class="mt-14 grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
			{#each communityCards as card (card.id)}
				<Card
					variant="white"
					shadow="md"
					padding="md"
					interactive={true}
					class="flex flex-col justify-between"
				>
					<div>
						<div class="flex items-center justify-between gap-2">
							<div
								class="flex h-11 w-11 items-center justify-center rounded-sm border-2 border-ink shadow-brutal-xs {getAccentClasses(card.accent)}"
							>
								{#if card.iconName === 'github'}
									<CodeXml class="h-5 w-5 stroke-[2.5]" />
								{:else if card.iconName === 'git-pull-request'}
									<GitPullRequest class="h-5 w-5 stroke-[2.5]" />
								{:else if card.iconName === 'bug'}
									<Bug class="h-5 w-5 stroke-[2.5]" />
								{:else}
									<MessagesSquare class="h-5 w-5 stroke-[2.5]" />
								{/if}
							</div>

							<Badge variant={card.accent} size="sm" shadow="none">
								{card.category}
							</Badge>
						</div>

						<h3 class="mt-5 font-heading text-lg font-extrabold text-ink sm:text-xl">
							{card.title}
						</h3>

						<p class="mt-2 text-xs sm:text-sm leading-relaxed text-ink/80">
							{card.description}
						</p>
					</div>

					<div class="mt-6 border-t-2 border-ink/15 pt-4">
						<a
							href={card.href}
							target={card.external ? '_blank' : undefined}
							rel={card.external ? 'noreferrer' : undefined}
							class="inline-flex items-center gap-1.5 font-heading text-xs sm:text-sm font-extrabold text-cobalt hover:text-ink focus-visible:outline-none focus-visible:underline"
						>
							<span>{card.ctaLabel}</span>
							<ArrowUpRight class="h-4 w-4 stroke-[2.5]" />
						</a>
					</div>
				</Card>
			{/each}
		</div>

		<!-- 5th Community Element: Community Edition v1.0 Roadmap -->
		<div
			class="mt-12 overflow-hidden rounded-md border-2 border-ink bg-surface shadow-brutal-lg"
		>
			<div
				class="flex flex-wrap items-center justify-between gap-4 border-b-2 border-ink bg-ink px-6 py-4 text-surface"
			>
				<div class="flex items-center gap-3">
					<div
						class="flex h-10 w-10 items-center justify-center rounded-sm border-2 border-surface bg-yellow text-ink"
					>
						<Map class="h-5 w-5 stroke-[2.5]" />
					</div>
					<div>
						<span class="font-mono text-[11px] font-bold uppercase tracking-widest text-yellow">
							DEVELOPMENT ROADMAP
						</span>
						<h3 class="font-heading text-lg font-extrabold text-surface sm:text-xl">
							Milestone: RiraLicense Community Edition v1.0
						</h3>
					</div>
				</div>

				<Button
					variant="yellow"
					size="sm"
					href={siteConfig.githubUrl}
					target="_blank"
					rel="noreferrer"
				>
					Follow Progress on GitHub
					<ArrowUpRight class="h-4 w-4 stroke-[2.5]" />
				</Button>
			</div>

			<div class="grid grid-cols-1 divide-y-2 divide-ink md:grid-cols-2 md:divide-y-0 lg:grid-cols-4 lg:divide-x-2">
				{#each communityRoadmap as item (item.id)}
					<div class="flex flex-col justify-between p-6">
						<div>
							<div class="flex items-center justify-between gap-2">
								<span class="font-mono text-[10px] font-extrabold uppercase tracking-wider text-ink/65">
									{item.module}
								</span>
								<Badge
									variant={getMilestoneStatusBadge(item.status)}
									size="sm"
									shadow="none"
								>
									{item.status}
								</Badge>
							</div>

							<h4 class="mt-4 font-heading text-base font-extrabold text-ink sm:text-lg">
								{item.title}
							</h4>

							<p class="mt-2 text-xs sm:text-sm leading-relaxed text-ink/80">
								{item.description}
							</p>
						</div>

						<div class="mt-5 flex items-center gap-1.5 border-t border-ink/15 pt-3 font-mono text-[11px] font-bold text-ink">
							{#if item.status === 'AVAILABLE'}
								<CheckCircle2 class="h-3.5 w-3.5 text-cobalt stroke-[2.5]" />
								<span>In Repository Now</span>
							{:else}
								<Clock class="h-3.5 w-3.5 text-coral stroke-[2.5]" />
								<span>Target: Community v1.0</span>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		</div>
	</div>
</section>
