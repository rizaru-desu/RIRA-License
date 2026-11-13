<script lang="ts">
	import {
		MonitorSmartphone,
		LayoutGrid,
		ServerCog,
		Check
	} from '@lucide/svelte';
	import SectionHeading from '$lib/components/ui/SectionHeading.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { supportedSoftwareList } from '$lib/data/features';

	function getHeaderClasses(accent: 'yellow' | 'cobalt' | 'coral'): string {
		switch (accent) {
			case 'yellow':
				return 'bg-yellow text-ink';
			case 'cobalt':
				return 'bg-cobalt text-surface';
			case 'coral':
				return 'bg-coral text-ink';
		}
	}
</script>

<section id="supported-software" class="border-b-2 border-ink bg-surface py-16 sm:py-24">
	<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
		<SectionHeading
			eyebrow="SUPPORTED SOFTWARE"
			badgeVariant="cobalt"
			title="One Licensing Engine Across Your Software Stack."
			description="RiraLicense provides a unified domain model for protecting compiled desktop binaries, web-based SaaS platforms, and backend API services."
		/>

		<div class="mt-14 grid grid-cols-1 gap-8 lg:grid-cols-3">
			{#each supportedSoftwareList as item (item.id)}
				<Card
					variant="cream"
					shadow="lg"
					padding="none"
					interactive={true}
					class="flex flex-col justify-between overflow-hidden"
				>
					<div>
						<div
							class="flex items-center justify-between border-b-2 border-ink px-6 py-4 {getHeaderClasses(item.accent)}"
						>
							<div class="flex items-center gap-3">
								<div
									class="flex h-11 w-11 items-center justify-center rounded-sm border-2 border-ink bg-surface text-ink shadow-brutal-xs"
								>
									{#if item.iconName === 'monitor-smartphone'}
										<MonitorSmartphone class="h-5 w-5 stroke-[2.5]" />
									{:else if item.iconName === 'layout-grid'}
										<LayoutGrid class="h-5 w-5 stroke-[2.5]" />
									{:else}
										<ServerCog class="h-5 w-5 stroke-[2.5]" />
									{/if}
								</div>
								<span class="font-mono text-xs font-extrabold uppercase tracking-wider">
									{item.category}
								</span>
							</div>

							<Badge variant="white" size="sm" shadow="none">
								{item.badgeText}
							</Badge>
						</div>

						<div class="p-6 sm:p-7">
							<div class="font-mono text-xs font-bold uppercase tracking-wider text-cobalt">
								{item.subtitle}
							</div>
							<h3 class="mt-1.5 font-heading text-2xl font-extrabold text-ink">
								{item.title}
							</h3>
							<p class="mt-3 text-sm sm:text-base leading-relaxed text-ink/80">
								{item.description}
							</p>

							<ul class="mt-6 space-y-2.5 border-t-2 border-ink/15 pt-5">
								{#each item.capabilities as capability (capability)}
									<li class="flex items-center gap-2.5 text-sm font-bold text-ink">
										<span
											class="flex h-5 w-5 shrink-0 items-center justify-center rounded-none border-2 border-ink {getHeaderClasses(item.accent)}"
										>
											<Check class="h-3 w-3 stroke-[3]" />
										</span>
										<span>{capability}</span>
									</li>
								{/each}
							</ul>
						</div>
					</div>

					<div
						class="flex items-center justify-between border-t-2 border-ink bg-surface px-6 py-3.5 font-mono text-xs font-bold text-ink/75"
					>
						<span>COMMUNITY EDITION CORE</span>
						<span class="font-extrabold text-ink">UNRESTRICTED</span>
					</div>
				</Card>
			{/each}
		</div>
	</div>
</section>
