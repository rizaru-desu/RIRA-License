<script lang="ts">
	import {
		ArrowRight,
		ArrowUpRight,
		CheckCircle2,
		Activity,
		Boxes,
		Cpu,
		KeyRound,
		ShieldCheck
	} from '@lucide/svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import {
		heroDashboardSummary,
		recentLicenseEvents,
		type LicenseStatus
	} from '$lib/data/features';
	import { siteConfig } from '$lib/data/navigation';

	let activeTab = $state<'inventory' | 'events'>('inventory');

	function getStatusVariant(status: LicenseStatus): 'cobalt' | 'yellow' | 'coral' | 'dark' {
		switch (status) {
			case 'ACTIVE':
				return 'cobalt';
			case 'TRIAL':
				return 'yellow';
			case 'SUSPENDED':
				return 'coral';
			case 'REVOKED':
				return 'dark';
		}
	}
</script>

<section
	id="top"
	class="relative overflow-hidden border-b-2 border-ink bg-cream bg-brutal-grid pt-8 pb-16 sm:pt-12 sm:pb-20 lg:pt-16 lg:pb-24"
>
	<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
		<div class="grid grid-cols-1 items-start gap-12 lg:grid-cols-12 lg:gap-10">
			<!-- Left Column: Positioning, Tagline, Supporting Message & CTAs -->
			<div class="flex flex-col items-start lg:col-span-6 lg:pt-2">
				<Badge variant="yellow" size="lg" shadow="sm" class="mb-6">
					<span class="inline-block h-2.5 w-2.5 border border-ink bg-cobalt"></span>
					OPEN-SOURCE SOFTWARE LICENSING INFRASTRUCTURE
				</Badge>

				<h1
					class="font-heading text-4xl font-extrabold tracking-tight text-ink sm:text-5xl lg:text-[54px] xl:text-[60px] leading-[1.04]"
				>
					<span class="block">Your Software.</span>
					<span class="mt-1 block">Your Licenses.</span>
					<span class="mt-1.5 block">
						<span
							class="inline-block rounded-md border-2 border-ink bg-yellow px-3 py-0.5 text-ink shadow-brutal-sm"
						>
							Your Control.
						</span>
					</span>
				</h1>

				<p class="mt-6 max-w-xl text-base sm:text-lg leading-relaxed text-ink/85">
					{siteConfig.supportingMessage}
				</p>

				<!-- Primary & Secondary CTAs -->
				<div class="mt-8 flex w-full flex-col gap-3.5 sm:w-auto sm:flex-row sm:items-center">
					<Button variant="primary" size="xl" href={siteConfig.getStartedHref}>
						Get Started
						<ArrowRight class="h-5 w-5 stroke-[2.5]" />
					</Button>

					<Button
						variant="secondary"
						size="xl"
						href={siteConfig.githubUrl}
						target="_blank"
						rel="noreferrer"
					>
						<svg
							class="h-4 w-4 shrink-0 fill-current"
							viewBox="0 0 24 24"
							aria-hidden="true"
						>
							<path
								fill-rule="evenodd"
								clip-rule="evenodd"
								d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"
							/>
						</svg>
						Explore on GitHub
						<ArrowUpRight class="h-4 w-4 stroke-[2.5]" />
					</Button>

					<div class="flex flex-col items-start gap-1">
						<span class="font-mono text-[10px] font-bold uppercase tracking-wider text-ink/70">
							STAR ON GITHUB
						</span>
						<a
							href="https://github.com/rizaru-desu/RIRA-License"
							target="_blank"
							rel="noreferrer"
						>
							<img
								src="https://img.shields.io/github/stars/rizaru-desu/RIRA-License?style=flat-square"
								alt="GitHub stars"
							/>
						</a>
					</div>
				</div>

				<!-- Core Identity Pillars: Open Source. Self Hosted. Developer First. -->
				<div
					class="mt-9 grid w-full grid-cols-1 gap-2.5 sm:grid-cols-3"
					aria-label="Core Product Principles"
				>
					<div
						class="flex items-center gap-2.5 rounded-md border-2 border-ink bg-surface px-3.5 py-2.5 shadow-brutal-xs"
					>
						<span
							class="flex h-6 w-6 shrink-0 items-center justify-center rounded-none border border-ink bg-yellow text-ink"
						>
							<CheckCircle2 class="h-3.5 w-3.5 stroke-[2.75]" />
						</span>
						<span class="font-heading text-xs font-extrabold uppercase tracking-wider text-ink">
							Open Source
						</span>
					</div>

					<div
						class="flex items-center gap-2.5 rounded-md border-2 border-ink bg-surface px-3.5 py-2.5 shadow-brutal-xs"
					>
						<span
							class="flex h-6 w-6 shrink-0 items-center justify-center rounded-none border border-ink bg-cobalt text-surface"
						>
							<CheckCircle2 class="h-3.5 w-3.5 stroke-[2.75]" />
						</span>
						<span class="font-heading text-xs font-extrabold uppercase tracking-wider text-ink">
							Self Hosted
						</span>
					</div>

					<div
						class="flex items-center gap-2.5 rounded-md border-2 border-ink bg-surface px-3.5 py-2.5 shadow-brutal-xs"
					>
						<span
							class="flex h-6 w-6 shrink-0 items-center justify-center rounded-none border border-ink bg-coral text-ink"
						>
							<CheckCircle2 class="h-3.5 w-3.5 stroke-[2.75]" />
						</span>
						<span class="font-heading text-xs font-extrabold uppercase tracking-wider text-ink">
							Developer First
						</span>
					</div>
				</div>
			</div>

			<!-- Right Column: Self-Hosted Product Dashboard Preview -->
			<div class="lg:col-span-6">
				<div
					class="overflow-hidden rounded-md border-2 border-ink bg-surface shadow-brutal-lg"
					aria-label="RiraLicense Community Edition Dashboard Preview"
				>
					<!-- Console Window Header -->
					<div
						class="flex flex-wrap items-center justify-between gap-2 border-b-2 border-ink bg-ink px-4 py-3 text-surface"
					>
						<div class="flex items-center gap-2">
							<span class="h-3.5 w-3.5 border border-surface bg-coral"></span>
							<span class="h-3.5 w-3.5 border border-surface bg-yellow"></span>
							<span class="h-3.5 w-3.5 border border-surface bg-cobalt"></span>
							<span class="ml-2 font-mono text-xs font-bold tracking-wider text-cream">
								riralicense://{heroDashboardSummary.instanceName}
							</span>
						</div>
						<div
							class="inline-flex items-center gap-1.5 rounded-sm border border-surface/40 bg-cobalt px-2.5 py-0.5 font-mono text-[11px] font-bold text-surface"
						>
							<span class="h-2 w-2 bg-yellow"></span>
							COMMUNITY EDITION • AGPL-3.0
						</div>
					</div>

					<div class="space-y-4 bg-cream/65 p-4 sm:p-5">
						<!-- Metrics Row: Active Licenses, Registered Products, Bound Devices -->
						<div class="grid grid-cols-3 gap-2.5 sm:gap-3">
							<div class="rounded-md border-2 border-ink bg-surface p-3 shadow-brutal-xs">
								<div class="flex items-center justify-between">
									<span class="font-mono text-[10px] font-bold uppercase tracking-wider text-ink/70">
										Active Licenses
									</span>
									<KeyRound class="h-3.5 w-3.5 text-cobalt stroke-[2.5]" />
								</div>
								<div class="mt-1 font-heading text-lg font-extrabold text-ink sm:text-2xl">
									{heroDashboardSummary.activeLicenses}
								</div>
								<span
									class="mt-1 inline-block rounded-none border border-ink bg-yellow px-1.5 py-0.5 font-mono text-[10px] font-bold text-ink"
								>
									SIGNED KEYS
								</span>
							</div>

							<div class="rounded-md border-2 border-ink bg-cobalt p-3 text-surface shadow-brutal-xs">
								<div class="flex items-center justify-between">
									<span class="font-mono text-[10px] font-bold uppercase tracking-wider text-cream">
										Products
									</span>
									<Boxes class="h-3.5 w-3.5 text-yellow stroke-[2.5]" />
								</div>
								<div class="mt-1 font-heading text-lg font-extrabold text-surface sm:text-2xl">
									{heroDashboardSummary.totalProducts}
								</div>
								<span
									class="mt-1 inline-block rounded-none border border-ink bg-surface px-1.5 py-0.5 font-mono text-[10px] font-bold text-ink"
								>
									DSK • SAAS • API
								</span>
							</div>

							<div class="rounded-md border-2 border-ink bg-surface p-3 shadow-brutal-xs">
								<div class="flex items-center justify-between">
									<span class="font-mono text-[10px] font-bold uppercase tracking-wider text-ink/70">
										Activations
									</span>
									<Cpu class="h-3.5 w-3.5 text-coral stroke-[2.5]" />
								</div>
								<div class="mt-1 font-heading text-lg font-extrabold text-ink sm:text-2xl">
									{heroDashboardSummary.activeActivations}
								</div>
								<span
									class="mt-1 inline-block rounded-none border border-ink bg-coral px-1.5 py-0.5 font-mono text-[10px] font-bold text-ink"
								>
									NODE-LOCKED
								</span>
							</div>
						</div>

						<!-- Validation Telemetry Strip -->
						<div class="rounded-md border-2 border-ink bg-surface p-3.5 shadow-brutal-xs">
							<div class="flex flex-wrap items-center justify-between gap-2">
								<div class="flex items-center gap-2">
									<Activity class="h-4 w-4 text-cobalt stroke-[2.5]" />
									<span class="font-heading text-xs font-extrabold uppercase tracking-wider text-ink">
										Self-Hosted Validation Pipeline
									</span>
								</div>
								<span class="font-mono text-xs font-extrabold text-cobalt">
									/healthz &amp; /readyz • {heroDashboardSummary.validationStatus}
								</span>
							</div>

							<div class="mt-2.5 grid grid-cols-12 gap-1 pt-1" aria-hidden="true">
								<div class="h-5 border border-ink bg-cobalt"></div>
								<div class="h-5 border border-ink bg-cobalt"></div>
								<div class="h-5 border border-ink bg-cobalt"></div>
								<div class="h-5 border border-ink bg-yellow"></div>
								<div class="h-5 border border-ink bg-cobalt"></div>
								<div class="h-5 border border-ink bg-cobalt"></div>
								<div class="h-5 border border-ink bg-cobalt"></div>
								<div class="h-5 border border-ink bg-cobalt"></div>
								<div class="h-5 border border-ink bg-yellow"></div>
								<div class="h-5 border border-ink bg-cobalt"></div>
								<div class="h-5 border border-ink bg-coral"></div>
								<div class="h-5 border border-ink bg-cobalt"></div>
							</div>
						</div>

						<!-- Product Inventory & Audit Log Tabs -->
						<div class="overflow-hidden rounded-md border-2 border-ink bg-surface shadow-brutal-xs">
							<div
								class="flex items-center justify-between border-b-2 border-ink bg-yellow px-3.5 py-2"
							>
								<div class="flex items-center gap-2" role="tablist" aria-label="Dashboard preview tabs">
									<button
										type="button"
										role="tab"
										aria-selected={activeTab === 'inventory'}
										onclick={() => (activeTab = 'inventory')}
										class="rounded-sm border border-ink px-2.5 py-1 font-heading text-[11px] font-extrabold uppercase tracking-wider transition-colors cursor-pointer {activeTab ===
										'inventory'
											? 'bg-ink text-yellow'
											: 'bg-surface text-ink hover:bg-cream'}"
									>
										Product Inventory
									</button>
									<button
										type="button"
										role="tab"
										aria-selected={activeTab === 'events'}
										onclick={() => (activeTab = 'events')}
										class="rounded-sm border border-ink px-2.5 py-1 font-heading text-[11px] font-extrabold uppercase tracking-wider transition-colors cursor-pointer {activeTab ===
										'events'
											? 'bg-ink text-yellow'
											: 'bg-surface text-ink hover:bg-cream'}"
									>
										Audit Log Events
									</button>
								</div>
								<span class="hidden sm:inline font-mono text-[10px] font-bold uppercase text-ink">
									POSTGRESQL ENGINE
								</span>
							</div>

							{#if activeTab === 'inventory'}
								<div class="overflow-x-auto">
									<table class="w-full border-collapse text-left">
										<thead>
											<tr
												class="border-b-2 border-ink bg-cream font-mono text-[10px] font-bold uppercase tracking-wider text-ink/80"
											>
												<th class="py-2 pl-3.5 pr-2">Product</th>
												<th class="px-2 py-2">Model</th>
												<th class="px-2 py-2">Activations</th>
												<th class="py-2 pl-2 pr-3.5 text-right">Status</th>
											</tr>
										</thead>
										<tbody class="divide-y-2 divide-ink/15 text-xs">
											{#each heroDashboardSummary.productInventory as prod (prod.id)}
												<tr class="transition-colors hover:bg-cream/60">
													<td class="py-2.5 pl-3.5 pr-2">
														<div class="font-heading font-bold text-ink">{prod.name}</div>
														<div class="font-mono text-[10px] text-ink/65">{prod.platform}</div>
													</td>
													<td class="px-2 py-2.5 font-mono text-[11px] font-bold text-ink whitespace-nowrap">
														{prod.licenseModel}
													</td>
													<td class="px-2 py-2.5 font-mono text-[11px] font-semibold text-ink/80 whitespace-nowrap">
														{prod.activations}
													</td>
													<td class="py-2.5 pl-2 pr-3.5 text-right whitespace-nowrap">
														<Badge
															variant={getStatusVariant(prod.status)}
															size="sm"
															shadow="none"
														>
															{prod.status}
														</Badge>
													</td>
												</tr>
											{/each}
										</tbody>
									</table>
								</div>
							{:else}
								<div class="divide-y-2 divide-ink/15">
									{#each recentLicenseEvents as evt (evt.id)}
										<div class="flex items-start justify-between gap-3 px-3.5 py-2.5 text-xs">
											<div class="min-w-0">
												<div class="flex items-center gap-2">
													<span class="font-mono font-extrabold text-cobalt">{evt.eventType}</span>
													<span class="font-mono text-[10px] text-ink/60">• {evt.key}</span>
												</div>
												<p class="mt-0.5 truncate text-ink/85 font-medium">{evt.description}</p>
											</div>
											<span class="shrink-0 font-mono text-[10px] font-bold text-ink/70">
												{evt.timestamp}
											</span>
										</div>
									{/each}
								</div>
							{/if}
						</div>

						<!-- Bottom Status Bar -->
						<div
							class="flex flex-wrap items-center justify-between gap-2 rounded-sm border-2 border-ink bg-ink px-3.5 py-2 font-mono text-[11px] text-cream"
						>
							<span class="inline-flex items-center gap-1.5 truncate">
								<ShieldCheck class="h-3.5 w-3.5 shrink-0 text-yellow" />
								<span>SELF-HOSTED ENGINE • ZERO EXTERNAL CLOUD DEPENDENCY</span>
							</span>
							<span class="text-yellow font-bold">PORT 7819</span>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</section>
