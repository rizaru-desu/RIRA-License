<script lang="ts">
	import {
		Container,
		Database,
		Settings2,
		HardDriveDownload,
		Check,
		BookOpen,
		ArrowUpRight,
		ArrowRight,
		CodeXml
	} from '@lucide/svelte';
	import SectionHeading from '$lib/components/ui/SectionHeading.svelte';
	import Card from '$lib/components/ui/Card.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import CodeBlock from '$lib/components/ui/CodeBlock.svelte';
	import {
		selfHostedPillars,
		selfHostedComposeSnippet,
		selfHostedEnvSnippet,
		developerCodeTabs,
		type SelfHostedPillar
	} from '$lib/data/features';
	import { siteConfig } from '$lib/data/navigation';

	const restValidationTab = developerCodeTabs[0];

	const prerequisitesSnippet = `# Verify Docker & Docker Compose
docker --version
docker compose version

# Optional: Run the Go + chi server directly from source (Go 1.22+)
cd server && go run ./cmd/api`;

	const cloneSnippet = `git clone ${siteConfig.githubUrl}.git
cd RIRA-License`;

	const startStackSnippet = `# Start the Go API server and PostgreSQL in detached mode
docker compose up -d`;

	const verifyHealthSnippet = `# Verify liveness and readiness endpoints on port 7819
curl http://localhost:7819/healthz
curl http://localhost:7819/readyz

# Expected /readyz JSON response:
# {"status":"ok","service":"riralicense-api","env":"production","uptime":"4m12s","timestamp":"2026-11-12T23:44:44+07:00"}`;

	function getIconBg(accent: SelfHostedPillar['accent']): string {
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

<section
	id="deployment"
	class="border-b-2 border-ink bg-surface bg-brutal-dots py-16 sm:py-24"
>
	<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
		<SectionHeading
			eyebrow="SELF-HOSTED DEPLOYMENT"
			badgeVariant="yellow"
			title="Deploy on Your Own Infrastructure."
			description="RiraLicense Community Edition runs independently on your own servers using Docker Compose and PostgreSQL—giving you complete ownership of your database, keys, and uptime."
		/>

		<!-- 4 Self-Hosted Deployment Pillars: Docker Compose, PostgreSQL, Environment Config, Backup & Maintenance -->
		<div class="mt-14 grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
			{#each selfHostedPillars as pillar (pillar.id)}
				<Card
					variant="cream"
					shadow="md"
					padding="md"
					interactive={true}
					class="flex flex-col justify-between"
				>
					<div>
						<div class="flex items-center justify-between gap-2">
							<div
								class="flex h-11 w-11 items-center justify-center rounded-sm border-2 border-ink shadow-brutal-xs {getIconBg(pillar.accent)}"
							>
								{#if pillar.iconName === 'container'}
									<Container class="h-5 w-5 stroke-[2.5]" />
								{:else if pillar.iconName === 'database'}
									<Database class="h-5 w-5 stroke-[2.5]" />
								{:else if pillar.iconName === 'settings-2'}
									<Settings2 class="h-5 w-5 stroke-[2.5]" />
								{:else}
									<HardDriveDownload class="h-5 w-5 stroke-[2.5]" />
								{/if}
							</div>

							<Badge variant={pillar.accent} size="sm" shadow="none">
								{pillar.badge}
							</Badge>
						</div>

						<h3 class="mt-5 font-heading text-xl font-extrabold text-ink">
							{pillar.title}
						</h3>

						<p class="mt-2 text-xs sm:text-sm leading-relaxed text-ink/80">
							{pillar.description}
						</p>
					</div>

					<ul class="mt-5 space-y-2 border-t-2 border-ink/15 pt-4">
						{#each pillar.details as detail (detail)}
							<li class="flex items-start gap-2 text-xs font-semibold text-ink">
								<span
									class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-none border border-ink bg-surface text-ink"
								>
									<Check class="h-2.5 w-2.5 stroke-[3]" />
								</span>
								<span>{detail}</span>
							</li>
						{/each}
					</ul>
				</Card>
			{/each}
		</div>

		<!-- Quick Start Sub-Header -->
		<div class="mt-16 flex flex-col items-start justify-between gap-4 border-t-2 border-ink pt-14 sm:flex-row sm:items-end">
			<div>
				<Badge variant="cobalt" size="md" shadow="xs">
					<span class="inline-block h-2 w-2 rounded-none bg-current"></span>
					QUICK START • STEPS 01–07
				</Badge>
				<h3 class="mt-3 font-heading text-2xl font-extrabold tracking-tight text-ink sm:text-3xl">
					From Zero to a Running Server &amp; Validated License.
				</h3>
			</div>
			<p class="max-w-md text-xs sm:text-sm font-medium leading-relaxed text-ink/80">
				Follow these seven ordered steps to configure, start, verify, and test your self-hosted RiraLicense instance.
			</p>
		</div>

		<!-- Numbered Quick-Start Steps (01 - 07) -->
		<div class="mt-8 grid grid-cols-1 gap-6 lg:grid-cols-2">
			<!-- Step 01: Prerequisites -->
			<Card
				variant="cream"
				shadow="lg"
				padding="none"
				interactive={true}
				class="flex flex-col justify-between overflow-hidden"
			>
				<div>
					<div
						class="flex items-center justify-between border-b-2 border-ink px-6 py-5 {getStepBoxClasses('yellow')}"
					>
						<span class="font-heading text-4xl font-extrabold tracking-tight sm:text-5xl leading-none">
							01
						</span>
						<div class="flex items-center gap-1.5">
							<span
								class="rounded-none border border-ink bg-surface px-2 py-0.5 font-mono text-[10px] font-extrabold uppercase tracking-wider text-ink"
							>
								STEP 01
							</span>
							<ArrowRight class="hidden h-4 w-4 stroke-[2.75] lg:inline" />
						</div>
					</div>

					<div class="p-6 space-y-5">
						<div>
							<div class="font-mono text-[11px] font-bold uppercase tracking-wider text-cobalt">
								SYSTEM REQUIREMENTS
							</div>
							<h3 class="mt-1.5 font-heading text-xl font-extrabold text-ink leading-snug">
								Prerequisites
							</h3>
							<p class="mt-3 text-sm leading-relaxed text-ink/80">
								Install Docker and Docker Compose for containerized deployment. Go (1.22+) is only
								required if running the server directly from source (<code class="rounded-sm border border-ink bg-surface px-1.5 py-0.5 font-mono text-xs font-bold text-ink">cd server &amp;&amp; go run ./cmd/api</code>).
							</p>
						</div>

						<CodeBlock
							title="Docker Compose & Optional Go Source Run"
							language="bash"
							badgeText="PREREQUISITES"
							badgeVariant="yellow"
							code={prerequisitesSnippet}
						/>
					</div>
				</div>

				<div class="border-t-2 border-ink bg-surface px-6 py-3 font-mono text-[11px] font-bold text-ink">
					STEP 01 • DOCKER + DOCKER COMPOSE READY
				</div>
			</Card>

			<!-- Step 02: Clone the Repository -->
			<Card
				variant="cream"
				shadow="lg"
				padding="none"
				interactive={true}
				class="flex flex-col justify-between overflow-hidden"
			>
				<div>
					<div
						class="flex items-center justify-between border-b-2 border-ink px-6 py-5 {getStepBoxClasses('cobalt')}"
					>
						<span class="font-heading text-4xl font-extrabold tracking-tight sm:text-5xl leading-none">
							02
						</span>
						<div class="flex items-center gap-1.5">
							<span
								class="rounded-none border border-ink bg-surface px-2 py-0.5 font-mono text-[10px] font-extrabold uppercase tracking-wider text-ink"
							>
								STEP 02
							</span>
							<ArrowRight class="hidden h-4 w-4 stroke-[2.75] lg:inline" />
						</div>
					</div>

					<div class="p-6 space-y-5">
						<div>
							<div class="font-mono text-[11px] font-bold uppercase tracking-wider text-cobalt">
								FETCH SOURCE CODE
							</div>
							<h3 class="mt-1.5 font-heading text-xl font-extrabold text-ink leading-snug">
								Clone the Repository
							</h3>
							<p class="mt-3 text-sm leading-relaxed text-ink/80">
								Clone the official RiraLicense repository from GitHub onto your local workstation,
								VPS, or on-premise Linux server.
							</p>
						</div>

						<CodeBlock
							title="Clone RiraLicense from GitHub"
							language="bash"
							badgeText="GIT CLONE"
							badgeVariant="cobalt"
							code={cloneSnippet}
						/>
					</div>
				</div>

				<div class="border-t-2 border-ink bg-surface px-6 py-3 font-mono text-[11px] font-bold text-ink">
					STEP 02 • REPOSITORY CLONED LOCALLY
				</div>
			</Card>

			<!-- Step 03: Configure Environment -->
			<Card
				variant="cream"
				shadow="lg"
				padding="none"
				interactive={true}
				class="flex flex-col justify-between overflow-hidden"
			>
				<div>
					<div
						class="flex items-center justify-between border-b-2 border-ink px-6 py-5 {getStepBoxClasses('coral')}"
					>
						<span class="font-heading text-4xl font-extrabold tracking-tight sm:text-5xl leading-none">
							03
						</span>
						<div class="flex items-center gap-1.5">
							<span
								class="rounded-none border border-ink bg-surface px-2 py-0.5 font-mono text-[10px] font-extrabold uppercase tracking-wider text-ink"
							>
								STEP 03
							</span>
							<ArrowRight class="hidden h-4 w-4 stroke-[2.75] lg:inline" />
						</div>
					</div>

					<div class="p-6 space-y-5">
						<div>
							<div class="font-mono text-[11px] font-bold uppercase tracking-wider text-cobalt">
								12-FACTOR CONFIGURATION
							</div>
							<h3 class="mt-1.5 font-heading text-xl font-extrabold text-ink leading-snug">
								Configure Environment
							</h3>
							<p class="mt-3 text-sm leading-relaxed text-ink/80">
								Copy the example environment configuration and set <code class="rounded-sm border border-ink bg-surface px-1.5 py-0.5 font-mono text-xs font-bold text-ink">APP_PORT</code> (default <code class="rounded-sm border border-ink bg-surface px-1.5 py-0.5 font-mono text-xs font-bold text-ink">7819</code>), <code class="rounded-sm border border-ink bg-surface px-1.5 py-0.5 font-mono text-xs font-bold text-ink">APP_ENV</code>, and <code class="rounded-sm border border-ink bg-surface px-1.5 py-0.5 font-mono text-xs font-bold text-ink">DATABASE_URL</code>.
							</p>
						</div>

						<CodeBlock
							title=".env & PostgreSQL Backup Workflow"
							language="bash"
							badgeText="ENV & BACKUP"
							badgeVariant="cobalt"
							code={selfHostedEnvSnippet}
						/>
					</div>
				</div>

				<div class="border-t-2 border-ink bg-surface px-6 py-3 font-mono text-[11px] font-bold text-ink">
					STEP 03 • APP_PORT, APP_ENV &amp; DATABASE_URL CONFIGURED
				</div>
			</Card>

			<!-- Step 04: Start the Stack -->
			<Card
				variant="cream"
				shadow="lg"
				padding="none"
				interactive={true}
				class="flex flex-col justify-between overflow-hidden"
			>
				<div>
					<div
						class="flex items-center justify-between border-b-2 border-ink px-6 py-5 {getStepBoxClasses('white')}"
					>
						<span class="font-heading text-4xl font-extrabold tracking-tight sm:text-5xl leading-none">
							04
						</span>
						<div class="flex items-center gap-1.5">
							<span
								class="rounded-none border border-ink bg-surface px-2 py-0.5 font-mono text-[10px] font-extrabold uppercase tracking-wider text-ink"
							>
								STEP 04
							</span>
							<ArrowRight class="hidden h-4 w-4 stroke-[2.75] lg:inline" />
						</div>
					</div>

					<div class="p-6 space-y-5">
						<div>
							<div class="font-mono text-[11px] font-bold uppercase tracking-wider text-cobalt">
								CONTAINER LAUNCH
							</div>
							<h3 class="mt-1.5 font-heading text-xl font-extrabold text-ink leading-snug">
								Start the Stack
							</h3>
							<p class="mt-3 text-sm leading-relaxed text-ink/80">
								Launch the RiraLicense Go API server and PostgreSQL database containers in detached
								mode using <code class="rounded-sm border border-ink bg-surface px-1.5 py-0.5 font-mono text-xs font-bold text-ink">docker compose up -d</code>.
							</p>
						</div>

						<CodeBlock
							title="Start Containers with Docker Compose"
							language="bash"
							badgeText="STARTUP"
							badgeVariant="white"
							code={startStackSnippet}
						/>

						<CodeBlock
							title="docker-compose.yml — Self-Hosted Stack (Go API + PostgreSQL)"
							language="yaml"
							badgeText="DOCKER COMPOSE"
							badgeVariant="yellow"
							code={selfHostedComposeSnippet}
						/>
					</div>
				</div>

				<div class="border-t-2 border-ink bg-surface px-6 py-3 font-mono text-[11px] font-bold text-ink">
					STEP 04 • DOCKER COMPOSE STACK UP ON PORT 7819
				</div>
			</Card>

			<!-- Step 05: Verify It Is Running -->
			<Card
				variant="cream"
				shadow="lg"
				padding="none"
				interactive={true}
				class="flex flex-col justify-between overflow-hidden"
			>
				<div>
					<div
						class="flex items-center justify-between border-b-2 border-ink px-6 py-5 {getStepBoxClasses('yellow')}"
					>
						<span class="font-heading text-4xl font-extrabold tracking-tight sm:text-5xl leading-none">
							05
						</span>
						<div class="flex items-center gap-1.5">
							<span
								class="rounded-none border border-ink bg-surface px-2 py-0.5 font-mono text-[10px] font-extrabold uppercase tracking-wider text-ink"
							>
								STEP 05
							</span>
							<ArrowRight class="hidden h-4 w-4 stroke-[2.75] lg:inline" />
						</div>
					</div>

					<div class="p-6 space-y-5">
						<div>
							<div class="font-mono text-[11px] font-bold uppercase tracking-wider text-cobalt">
								HEALTH &amp; READINESS PROBES
							</div>
							<h3 class="mt-1.5 font-heading text-xl font-extrabold text-ink leading-snug">
								Verify It Is Running
							</h3>
							<p class="mt-3 text-sm leading-relaxed text-ink/80">
								Query <code class="rounded-sm border border-ink bg-surface px-1.5 py-0.5 font-mono text-xs font-bold text-ink">http://localhost:7819/healthz</code> and <code class="rounded-sm border border-ink bg-surface px-1.5 py-0.5 font-mono text-xs font-bold text-ink">http://localhost:7819/readyz</code>. Both endpoints exist in the Go server and reply with <code class="rounded-sm border border-ink bg-surface px-1.5 py-0.5 font-mono text-xs font-bold text-ink">{`{"status": "ok", ...}`}</code>.
							</p>
						</div>

						<CodeBlock
							title="Probe /healthz & /readyz on Port 7819"
							language="bash"
							badgeText="HEALTHZ & READYZ"
							badgeVariant="coral"
							code={verifyHealthSnippet}
						/>
					</div>
				</div>

				<div class="border-t-2 border-ink bg-surface px-6 py-3 font-mono text-[11px] font-bold text-ink">
					STEP 05 • SERVER HEALTH &amp; READINESS VERIFIED
				</div>
			</Card>

			<!-- Step 06: Validate a License -->
			<Card
				variant="cream"
				shadow="lg"
				padding="none"
				interactive={true}
				class="flex flex-col justify-between overflow-hidden"
			>
				<div>
					<div
						class="flex items-center justify-between border-b-2 border-ink px-6 py-5 {getStepBoxClasses('cobalt')}"
					>
						<span class="font-heading text-4xl font-extrabold tracking-tight sm:text-5xl leading-none">
							06
						</span>
						<div class="flex items-center gap-1.5">
							<span
								class="rounded-none border border-ink bg-surface px-2 py-0.5 font-mono text-[10px] font-extrabold uppercase tracking-wider text-ink"
							>
								STEP 06
							</span>
							<ArrowRight class="hidden h-4 w-4 stroke-[2.75] lg:inline" />
						</div>
					</div>

					<div class="p-6 space-y-5">
						<div>
							<div class="font-mono text-[11px] font-bold uppercase tracking-wider text-cobalt">
								REST VALIDATION ENDPOINT
							</div>
							<h3 class="mt-1.5 font-heading text-xl font-extrabold text-ink leading-snug">
								Validate a License
							</h3>
							<p class="mt-3 text-sm leading-relaxed text-ink/80">
								Send a <code class="rounded-sm border border-ink bg-surface px-1.5 py-0.5 font-mono text-xs font-bold text-ink">POST /api/v1/validate</code> request using the contract from the
								<a
									href="#developer-experience"
									class="font-bold text-cobalt underline decoration-2 underline-offset-4 hover:text-ink"
								>
									Developer Experience
								</a>
								section to verify a license key, device fingerprint, and feature entitlements.
							</p>
						</div>

						<CodeBlock
							title={restValidationTab.title}
							language={restValidationTab.language}
							badgeText={restValidationTab.statusBadge}
							badgeVariant={restValidationTab.statusVariant}
							code={restValidationTab.code}
						/>

						<CodeBlock
							title="Response Payload / Verification Output"
							language="json"
							badgeText="OUTPUT"
							badgeVariant="white"
							code={restValidationTab.responsePreview}
						/>
					</div>
				</div>

				<div class="border-t-2 border-ink bg-surface px-6 py-3 font-mono text-[11px] font-bold text-ink">
					STEP 06 • LICENSE VALIDATED VIA POST /API/V1/VALIDATE
				</div>
			</Card>

			<!-- Step 07: Next Steps (Full Width) -->
			<Card
				variant="cream"
				shadow="lg"
				padding="none"
				interactive={true}
				class="flex flex-col justify-between overflow-hidden lg:col-span-2"
			>
				<div>
					<div
						class="flex items-center justify-between border-b-2 border-ink px-6 py-5 {getStepBoxClasses('coral')}"
					>
						<span class="font-heading text-4xl font-extrabold tracking-tight sm:text-5xl leading-none">
							07
						</span>
						<div class="flex items-center gap-1.5">
							<span
								class="rounded-none border border-ink bg-surface px-2 py-0.5 font-mono text-[10px] font-extrabold uppercase tracking-wider text-ink"
							>
								STEP 07
							</span>
						</div>
					</div>

					<div class="p-6">
						<div class="grid grid-cols-1 gap-6 lg:grid-cols-12 lg:items-center">
							<div class="space-y-4 lg:col-span-6">
								<div>
									<div class="font-mono text-[11px] font-bold uppercase tracking-wider text-cobalt">
										CONTINUE BUILDING
									</div>
									<h3 class="mt-1.5 font-heading text-xl font-extrabold text-ink leading-snug">
										Next Steps
									</h3>
									<p class="mt-3 text-sm leading-relaxed text-ink/80">
										Dive deeper into the RiraLicense architecture in the GitHub repository README,
										explore the SDK Integration tab in Developer Experience, or review the complete
										Deployment Guide.
									</p>
								</div>

								<div class="flex flex-wrap items-center gap-3 pt-1">
									<Button
										variant="primary"
										size="md"
										href={siteConfig.githubReadmeUrl}
										target="_blank"
										rel="noreferrer"
									>
										<BookOpen class="h-4 w-4 stroke-[2.25]" />
										GitHub README &amp; Docs
										<ArrowUpRight class="h-4 w-4 stroke-[2.5]" />
									</Button>

									<Button
										variant="secondary"
										size="md"
										href={siteConfig.apiReferenceHref}
									>
										<CodeXml class="h-4 w-4 stroke-[2.25]" />
										SDK Integration Tab
										<ArrowRight class="h-4 w-4 stroke-[2.5]" />
									</Button>

									<Button
										variant="yellow"
										size="md"
										href={siteConfig.githubReadmeUrl}
										target="_blank"
										rel="noreferrer"
									>
										Deployment Guide
										<ArrowUpRight class="h-4 w-4 stroke-[2.5]" />
									</Button>
								</div>
							</div>

							<!-- Existing Yellow Installation & Operations Guide Card -->
							<div class="lg:col-span-6">
								<div
									class="flex flex-col justify-between gap-4 rounded-md border-2 border-ink bg-yellow p-6 shadow-brutal"
								>
									<div>
										<div class="font-mono text-xs font-extrabold uppercase tracking-wider text-ink/80">
											INSTALLATION &amp; OPERATIONS GUIDE
										</div>
										<h4 class="mt-1 font-heading text-xl font-extrabold text-ink">
											Ready to Deploy RiraLicense Locally or on a VPS?
										</h4>
										<p class="mt-1.5 text-xs sm:text-sm font-medium leading-relaxed text-ink/85">
											Read the full repository documentation for Go server startup, frontend development,
											environment variables, and health check verification.
										</p>
									</div>

									<div class="flex flex-wrap items-center gap-3 pt-1">
										<Button
											variant="dark"
											size="md"
											href={siteConfig.githubReadmeUrl}
											target="_blank"
											rel="noreferrer"
										>
											<BookOpen class="h-4 w-4 stroke-[2.25]" />
											Read Deployment Documentation
											<ArrowUpRight class="h-4 w-4 stroke-[2.5]" />
										</Button>
									</div>
								</div>
							</div>
						</div>
					</div>
				</div>

				<div class="border-t-2 border-ink bg-surface px-6 py-3 font-mono text-[11px] font-bold text-ink">
					STEP 07 • DOCS, SDK INTEGRATION &amp; DEPLOYMENT GUIDE
				</div>
			</Card>
		</div>
	</div>
</section>
