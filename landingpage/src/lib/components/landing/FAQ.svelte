<script lang="ts">
	import { Accordion } from 'bits-ui';
	import { Plus, Minus, HelpCircle } from '@lucide/svelte';
	import SectionHeading from '$lib/components/ui/SectionHeading.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import { faqItems } from '$lib/data/faq';

	let activeFaq = $state<string>('faq-1');
</script>

<section id="faq" class="border-b-2 border-ink bg-surface py-16 sm:py-24">
	<div class="mx-auto max-w-4xl px-4 sm:px-6 lg:px-8">
		<SectionHeading
			eyebrow="FREQUENTLY ASKED QUESTIONS"
			badgeVariant="coral"
			title="Open-Source & Self-Hosting FAQ."
			description="Answers to common questions about RiraLicense Community Edition, self-hosting, supported license types, offline verification, and documentation."
		/>

		<div class="mt-12">
			<Accordion.Root type="single" bind:value={activeFaq} class="space-y-4">
				{#each faqItems as item, idx (item.id)}
					{@const isOpen = activeFaq === item.id}
					{@const itemNum = String(idx + 1).padStart(2, '0')}
					<Accordion.Item
						value={item.id}
						class="overflow-hidden rounded-md border-2 border-ink bg-surface transition-all duration-150 {isOpen
							? 'shadow-brutal-md -translate-y-0.5'
							: 'shadow-brutal hover:bg-cream/40'}"
					>
						<Accordion.Header>
							<Accordion.Trigger
								class="flex w-full items-center justify-between gap-4 px-5 py-5 text-left transition-colors sm:px-6 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cobalt {isOpen
									? 'border-b-2 border-ink bg-yellow text-ink'
									: 'bg-surface text-ink hover:bg-cream/60'}"
							>
								<div class="flex flex-col gap-1.5 sm:flex-row sm:items-center sm:gap-3">
									<Badge
										variant={isOpen ? 'white' : 'cream'}
										size="sm"
										shadow="none"
										class="w-fit shrink-0"
									>
										{itemNum} • {item.category}
									</Badge>
									<span class="font-heading text-base font-extrabold sm:text-lg">
										{item.question}
									</span>
								</div>

								<span
									class="flex h-9 w-9 shrink-0 items-center justify-center rounded-sm border-2 border-ink {isOpen
										? 'bg-ink text-yellow'
										: 'bg-cream text-ink'} shadow-brutal-xs"
									aria-hidden="true"
								>
									{#if isOpen}
										<Minus class="h-4 w-4 stroke-[2.75]" />
									{:else}
										<Plus class="h-4 w-4 stroke-[2.75]" />
									{/if}
								</span>
							</Accordion.Trigger>
						</Accordion.Header>

						<Accordion.Content
							class="bg-cream/40 px-5 py-5 text-sm sm:px-6 sm:text-base leading-relaxed text-ink/85 data-[state=closed]:hidden"
						>
							<div class="flex gap-3">
								<HelpCircle class="mt-0.5 hidden h-5 w-5 shrink-0 text-cobalt stroke-[2.25] sm:block" />
								<p>{item.answer}</p>
							</div>
						</Accordion.Content>
					</Accordion.Item>
				{/each}
			</Accordion.Root>
		</div>
	</div>
</section>
