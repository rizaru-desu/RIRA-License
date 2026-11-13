<script lang="ts">
	import {
		BookOpen,
		ArrowUpRight,
		Terminal,
		ShieldCheck,
		Layers,
		CodeXml
	} from '@lucide/svelte';
	import SectionHeading from '$lib/components/ui/SectionHeading.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import CodeBlock from '$lib/components/ui/CodeBlock.svelte';
	import { developerCodeTabs, type DeveloperCodeTab } from '$lib/data/features';
	import { siteConfig } from '$lib/data/navigation';

	let selectedTabId = $state<DeveloperCodeTab['id']>('rest-api');

	let activeTab = $derived(
		developerCodeTabs.find((tab) => tab.id === selectedTabId) ?? developerCodeTabs[0]
	);
</script>

<section
	id="developer-experience"
	class="border-b-2 border-ink bg-cream bg-brutal-grid py-16 sm:py-24"
>
	<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
		<SectionHeading
			eyebrow="DEVELOPER EXPERIENCE & API REFERENCE"
			badgeVariant="cobalt"
			title="Clean Contracts. Modular Architecture."
			description="Built with a disciplined Go + chi backend and Svelte 5 frontend. Integrate license validation via straightforward HTTP endpoints or planned client SDKs."
		/>

		<div class="mt-14 grid grid-cols-1 items-start gap-8 lg:grid-cols-12 lg:gap-10">
			<!-- Left Column: Developer-Friendly Workflows & Architecture -->
			<div class="space-y-6 lg:col-span-5">
				<div class="rounded-md border-2 border-ink bg-surface p-6 shadow-brutal">
					<Badge variant="yellow" size="sm" shadow="xs">
						MODULAR MONOLITH ARCHITECTURE
					</Badge>

					<h3 class="mt-4 font-heading text-2xl font-extrabold text-ink">
						Engineered for Maintainability &amp; Speed
					</h3>

					<p class="mt-2.5 text-sm sm:text-base leading-relaxed text-ink/80">
						RiraLicense separates core licensing domain rules from infrastructure adapters—keeping
						the codebase clean, testable, and completely independent of external cloud services.
					</p>

					<div class="mt-6 space-y-4 border-t-2 border-ink/15 pt-5">
						<div class="flex items-start gap-3">
							<div
								class="flex h-9 w-9 shrink-0 items-center justify-center rounded-sm border-2 border-ink bg-yellow text-ink shadow-brutal-xs"
							>
								<Terminal class="h-4 w-4 stroke-[2.5]" />
							</div>
							<div>
								<h4 class="font-heading text-sm font-extrabold text-ink">
									REST API Contracts
								</h4>
								<p class="mt-0.5 text-xs leading-relaxed text-ink/75">
									Standard JSON request/response payloads for license validation, device activation,
									and health monitoring.
								</p>
							</div>
						</div>

						<div class="flex items-start gap-3">
							<div
								class="flex h-9 w-9 shrink-0 items-center justify-center rounded-sm border-2 border-ink bg-cobalt text-surface shadow-brutal-xs"
							>
								<ShieldCheck class="h-4 w-4 stroke-[2.5]" />
							</div>
							<div>
								<h4 class="font-heading text-sm font-extrabold text-ink">
									Asymmetric Digital Signatures
								</h4>
								<p class="mt-0.5 text-xs leading-relaxed text-ink/75">
									Embed only your public key in client binaries to verify signed offline grants
									safely.
								</p>
							</div>
						</div>

						<div class="flex items-start gap-3">
							<div
								class="flex h-9 w-9 shrink-0 items-center justify-center rounded-sm border-2 border-ink bg-coral text-ink shadow-brutal-xs"
							>
								<Layers class="h-4 w-4 stroke-[2.5]" />
							</div>
							<div>
								<h4 class="font-heading text-sm font-extrabold text-ink">
									Clean Go + chi Layering (`handler` → `service` → `repo`)
								</h4>
								<p class="mt-0.5 text-xs leading-relaxed text-ink/75">
									Built with RequestID, RealIP, Logger, Recoverer, and Timeout middleware out of the
									box.
								</p>
							</div>
						</div>
					</div>

					<div class="mt-7 flex flex-col gap-3 sm:flex-row">
						<Button variant="primary" size="md" href={siteConfig.documentationHref} class="flex-1">
							<BookOpen class="h-4 w-4 stroke-[2.25]" />
							Deployment Guide
						</Button>

						<Button
							variant="secondary"
							size="md"
							href={siteConfig.githubUrl}
							target="_blank"
							rel="noreferrer"
							class="flex-1"
						>
							<CodeXml class="h-4 w-4 stroke-[2.25]" />
							Inspect Source
							<ArrowUpRight class="h-4 w-4 stroke-[2.5]" />
						</Button>
					</div>
				</div>
			</div>

			<!-- Right Column: Code Examples (REST API, SDK Preview, Live Probe) -->
			<div class="space-y-4 lg:col-span-7">
				<div
					class="flex flex-wrap gap-2 rounded-md border-2 border-ink bg-surface p-2 shadow-brutal-sm"
					role="tablist"
					aria-label="Developer code examples"
				>
					{#each developerCodeTabs as tab (tab.id)}
						<button
							type="button"
							role="tab"
							aria-selected={selectedTabId === tab.id}
							onclick={() => (selectedTabId = tab.id)}
							class="flex-1 rounded-sm border-2 px-3.5 py-2.5 font-heading text-xs sm:text-sm font-extrabold transition-all cursor-pointer whitespace-nowrap {selectedTabId ===
							tab.id
								? 'border-ink bg-yellow text-ink shadow-brutal-xs -translate-y-0.5'
								: 'border-transparent bg-cream/60 text-ink/80 hover:border-ink hover:bg-cream'}"
						>
							{tab.label}
						</button>
					{/each}
				</div>

				<div
					class="flex flex-wrap items-center justify-between gap-2 rounded-md border-2 border-ink bg-surface px-4 py-3 shadow-brutal-xs"
				>
					<p class="text-xs sm:text-sm font-semibold text-ink/85">
						{activeTab.description}
					</p>
				</div>

				<CodeBlock
					title={activeTab.title}
					language={activeTab.language}
					badgeText={activeTab.statusBadge}
					badgeVariant={activeTab.statusVariant}
					code={activeTab.code}
				/>

				<CodeBlock
					title="Response Payload / Verification Output"
					language="json"
					badgeText="OUTPUT"
					badgeVariant="white"
					code={activeTab.responsePreview}
				/>
			</div>
		</div>
	</div>
</section>
