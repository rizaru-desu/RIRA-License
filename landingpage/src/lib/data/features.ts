export type AccentColor = 'yellow' | 'cobalt' | 'coral' | 'white' | 'cream';

export interface SupportedSoftwareItem {
	id: string;
	category: string;
	title: string;
	subtitle: string;
	description: string;
	capabilities: string[];
	badgeText: string;
	accent: 'yellow' | 'cobalt' | 'coral';
	iconName: 'monitor-smartphone' | 'layout-grid' | 'server-cog';
}

export interface CoreFeatureItem {
	id: string;
	number: string;
	title: string;
	tag: string;
	statusLabel: 'AVAILABLE FOUNDATION' | 'IN DEVELOPMENT (v1.0)' | 'PLANNED (v1.0)';
	description: string;
	highlights: string[];
	accent: 'yellow' | 'cobalt' | 'coral' | 'white';
	iconName:
		| 'key-round'
		| 'check-circle-2'
		| 'cpu'
		| 'sliders-horizontal'
		| 'shield-check'
		| 'webhook';
}

export interface HowItWorksStep {
	step: string;
	title: string;
	subtitle: string;
	description: string;
	deliverable: string;
	accent: 'yellow' | 'cobalt' | 'coral' | 'white';
}

export interface DeveloperCodeTab {
	id: 'rest-api' | 'sdk-integration' | 'health-probe';
	label: string;
	title: string;
	language: string;
	statusBadge: string;
	statusVariant: 'yellow' | 'cobalt' | 'coral' | 'white';
	description: string;
	code: string;
	responsePreview: string;
}

export interface SelfHostedPillar {
	id: string;
	title: string;
	badge: string;
	description: string;
	details: string[];
	accent: 'yellow' | 'cobalt' | 'coral' | 'white';
	iconName: 'container' | 'database' | 'settings-2' | 'hard-drive-download';
}

export type LicenseStatus = 'ACTIVE' | 'TRIAL' | 'SUSPENDED' | 'REVOKED';

export interface LicenseEventItem {
	id: string;
	timestamp: string;
	eventType: string;
	description: string;
	key: string;
	product: string;
	status: 'VALID' | 'ACTIVATED' | 'AUDIT' | 'REVOKED';
}

export const supportedSoftwareList: SupportedSoftwareItem[] = [
	{
		id: 'desktop-applications',
		category: '01 • DESKTOP SOFTWARE',
		title: 'Desktop Applications',
		subtitle: 'Windows, macOS & Linux Binaries',
		description:
			'Protect native and cross-platform desktop applications with node-locked device activations, hardware fingerprinting, and offline digital signature verification.',
		capabilities: [
			'Perpetual, subscription & trial licenses',
			'Node-locked device activation & deactivation',
			'Offline license verification via digital signatures',
			'Feature-based entitlements per edition'
		],
		badgeText: 'WIN • MAC • LINUX',
		accent: 'yellow',
		iconName: 'monitor-smartphone'
	},
	{
		id: 'saas-applications',
		category: '02 • WEB & SAAS PLATFORMS',
		title: 'SaaS Applications',
		subtitle: 'Self-Hosted & Multi-Tenant Applications',
		description:
			'Manage customer license lifecycles, validate active subscriptions or trials, and gate product modules using structured feature entitlements.',
		capabilities: [
			'Subscription & trial lifecycle validation',
			'Feature-based entitlements & quota flags',
			'Webhook notifications for state changes',
			'Centralized audit logs for compliance'
		],
		badgeText: 'WEB • B2B • ON-PREM',
		accent: 'cobalt',
		iconName: 'layout-grid'
	},
	{
		id: 'apis-backend-services',
		category: '03 • BACKEND & DEV TOOLS',
		title: 'APIs and Backend Services',
		subtitle: 'Microservices, CLI Tools & Workers',
		description:
			'Authenticate commercial APIs, CLI binaries, and background workers through low-latency REST validation endpoints and deterministic JSON responses.',
		capabilities: [
			'Stateless REST API license validation',
			'Service & runner activation tracking',
			'Developer-friendly HTTP & SDK workflows',
			'Zero dependency on third-party cloud vendors'
		],
		badgeText: 'REST • CLI • DAEMON',
		accent: 'coral',
		iconName: 'server-cog'
	}
];

export const coreFeaturesList: CoreFeatureItem[] = [
	{
		id: 'license-management',
		number: '01',
		title: 'License Management',
		tag: 'LIFECYCLE ENGINE',
		statusLabel: 'IN DEVELOPMENT (v1.0)',
		description:
			'Generate, issue, renew, suspend, and revoke software license keys across perpetual, subscription, and trial policies with complete audit logs.',
		highlights: [
			'Perpetual, subscription, and trial license models',
			'Full license lifecycle management (issue, suspend, revoke)',
			'Immutable audit logs for every state transition'
		],
		accent: 'yellow',
		iconName: 'key-round'
	},
	{
		id: 'license-validation',
		number: '02',
		title: 'License Validation',
		tag: 'VALIDATION API',
		statusLabel: 'IN DEVELOPMENT (v1.0)',
		description:
			'Verify license authenticity, expiration timestamps, and policy constraints in real time through a fast, predictable HTTP validation endpoint.',
		highlights: [
			'Deterministic JSON validation responses',
			'Policy & expiration window verification',
			'Powered by a lightweight Go + chi HTTP server'
		],
		accent: 'cobalt',
		iconName: 'check-circle-2'
	},
	{
		id: 'device-activation',
		number: '03',
		title: 'Device Activation',
		tag: 'NODE-LOCKED CONTROL',
		statusLabel: 'IN DEVELOPMENT (v1.0)',
		description:
			'Enforce node-locked licensing by binding license keys to specific device fingerprints and controlling activation and deactivation slots.',
		highlights: [
			'Node-locked machine fingerprint binding',
			'Device activation & deactivation endpoints',
			'Configurable maximum device limits per license'
		],
		accent: 'coral',
		iconName: 'cpu'
	},
	{
		id: 'entitlements',
		number: '04',
		title: 'Entitlements',
		tag: 'FEATURE GATING',
		statusLabel: 'IN DEVELOPMENT (v1.0)',
		description:
			'Define feature-based entitlements on your products and attach them to licenses so your application can enable or restrict capabilities dynamically.',
		highlights: [
			'Feature-based boolean & key-value entitlements',
			'Decouple product capabilities from application code',
			'Returned automatically in validation payloads'
		],
		accent: 'white',
		iconName: 'sliders-horizontal'
	},
	{
		id: 'offline-verification',
		number: '05',
		title: 'Offline Verification',
		tag: 'DIGITAL SIGNATURES',
		statusLabel: 'PLANNED (v1.0)',
		description:
			'Verify cryptographically signed license files and grants using asymmetric digital signatures—allowing software to run in air-gapped or offline environments.',
		highlights: [
			'Asymmetric digital signature verification (Ed25519)',
			'Public-key verification safe for client binaries',
			'Zero constant internet requirement after activation'
		],
		accent: 'yellow',
		iconName: 'shield-check'
	},
	{
		id: 'developer-apis',
		number: '06',
		title: 'Developer APIs',
		tag: 'REST, WEBHOOKS & SDKs',
		statusLabel: 'AVAILABLE FOUNDATION',
		description:
			'Built on a clean Go + chi REST architecture with health probes today, expanding in v1.0 with lifecycle webhooks and official developer SDKs.',
		highlights: [
			'Live /healthz & /readyz server endpoints available now',
			'REST API & webhook event contracts (v1.0 milestone)',
			'Official Developer SDKs (Planned for v1.0)'
		],
		accent: 'cobalt',
		iconName: 'webhook'
	}
];

export const howItWorksSteps: HowItWorksStep[] = [
	{
		step: '01',
		title: 'Create a Product',
		subtitle: 'Define Policies & Entitlements',
		description:
			'Register your desktop application, SaaS product, or backend service in your self-hosted RiraLicense instance and configure its license policies and feature entitlements.',
		deliverable: 'STEP 01 • PRODUCT & POLICY DEFINED',
		accent: 'yellow'
	},
	{
		step: '02',
		title: 'Generate a License',
		subtitle: 'Perpetual, Subscription, or Trial',
		description:
			'Create and issue license keys for your users or customers—configuring expiration rules, node-locked device limits, and attached entitlements.',
		deliverable: 'STEP 02 • LICENSE KEY ISSUED',
		accent: 'cobalt'
	},
	{
		step: '03',
		title: 'Integrate the SDK or API',
		subtitle: 'Connect Your Software',
		description:
			'Add a clean REST API request or integrate the planned client SDK into your application startup or request middleware.',
		deliverable: 'STEP 03 • CLIENT INTEGRATED',
		accent: 'coral'
	},
	{
		step: '04',
		title: 'Validate and Activate Licenses',
		subtitle: 'Enforce & Audit in Real Time',
		description:
			'Validate keys online or verify signed offline grants, bind activated device fingerprints, unlock entitled features, and track every event in audit logs.',
		deliverable: 'STEP 04 • ENFORCED & AUDITED',
		accent: 'white'
	}
];

export const developerCodeTabs: DeveloperCodeTab[] = [
	{
		id: 'rest-api',
		label: '01. REST API Validation',
		title: 'POST /api/v1/validate (v1.0 Contract Spec)',
		language: 'http',
		statusBadge: 'REST API SPEC',
		statusVariant: 'yellow',
		description:
			'Validate a license key, register or check a node-locked device fingerprint, and retrieve feature-based entitlements over standard HTTP.',
		code: `POST /api/v1/validate HTTP/1.1
Host: licenses.internal.example.com:7819
Content-Type: application/json
Accept: application/json

{
  "productId": "prod_desktop_studio",
  "licenseKey": "RIRA-XXXX-XXXX-XXXX",
  "deviceFingerprint": "sha256:9f8a4b2c7d1e...",
  "deviceName": "workstation-linux-01"
}`,
		responsePreview: `{
  "valid": true,
  "status": "ACTIVE",
  "licenseType": "PERPETUAL",
  "deviceActivation": {
    "bound": true,
    "activationsUsed": 1,
    "activationsMax": 2
  },
  "entitlements": ["pro_export", "offline_signature"],
  "expiresAt": null
}`
	},
	{
		id: 'sdk-integration',
		label: '02. SDK Integration',
		title: 'validate-license.ts (Planned v1.0 SDK Interface)',
		language: 'typescript',
		statusBadge: 'PLANNED v1.0 SDK',
		statusVariant: 'cobalt',
		description:
			'Integrate license verification and offline digital signature checks using a clean, strongly-typed SDK interface.',
		code: `import { RiraLicense } from "@riralicense/sdk";

// Public verification key for offline digital signature checks
const PUBLIC_KEY = "MCowBQYDK2VwAyEA8f9b2c...";

const license = new RiraLicense({
  baseUrl: "http://localhost:7819",
  productId: "prod_desktop_studio",
  publicKey: PUBLIC_KEY
});

const result = await license.validate({
  licenseKey: "RIRA-XXXX-XXXX-XXXX",
  deviceFingerprint: "sha256:9f8a4b2c7d1e..."
});

if (result.valid) {
  console.log("Entitled features:", result.entitlements);
}`,
		responsePreview: `// Output:
// Entitled features: ["pro_export", "offline_signature"]
// Digital signature verified against Ed25519 PUBLIC_KEY`
	},
	{
		id: 'health-probe',
		label: '03. Live Go Server Probe',
		title: 'GET /readyz (Implemented in server/)',
		language: 'bash',
		statusBadge: 'AVAILABLE NOW',
		statusVariant: 'coral',
		description:
			'The Go + chi server in server/ already provides production liveness (/healthz) and readiness (/readyz) endpoints with graceful shutdown.',
		code: `# Start the RiraLicense Go + chi server locally
$ cd server && go run ./cmd/api

# Query the live readiness endpoint on port 7819
$ curl -i http://localhost:7819/readyz`,
		responsePreview: `HTTP/1.1 200 OK
Content-Type: application/json

{
  "status": "ok",
  "service": "riralicense-api",
  "env": "development",
  "uptime": "4m12s",
  "timestamp": "2026-11-12T23:44:44+07:00"
}`
	}
];

export const selfHostedPillars: SelfHostedPillar[] = [
	{
		id: 'docker-compose',
		title: 'Docker Compose',
		badge: 'CONTAINER STACK',
		description:
			'Deploy the RiraLicense Go API server and web console on any Linux host, VPS, or on-premise server using a standard Docker Compose configuration.',
		details: [
			'Single-command startup (docker compose up -d)',
			'Lightweight Go binary + static frontend bundle',
			'Built-in container healthchecks via /healthz'
		],
		accent: 'yellow',
		iconName: 'container'
	},
	{
		id: 'postgresql',
		title: 'PostgreSQL',
		badge: 'RELATIONAL STORAGE',
		description:
			'Store all products, license keys, device activations, entitlements, and audit logs in your own self-managed PostgreSQL database.',
		details: [
			'Full data ownership with zero third-party lock-in',
			'Deterministic SQL schema migrations',
			'ACID guarantees for activation concurrency'
		],
		accent: 'cobalt',
		iconName: 'database'
	},
	{
		id: 'environment-config',
		title: 'Environment Configuration',
		badge: '12-FACTOR CONFIG',
		description:
			'Configure ports, runtime environments, database connections, and cryptographic signing keys through explicit environment variables.',
		details: [
			'APP_PORT (default 7819) & APP_ENV (development / production)',
			'Explicit DATABASE_URL & keypair configuration',
			'No hidden cloud callbacks or telemetry'
		],
		accent: 'coral',
		iconName: 'settings-2'
	},
	{
		id: 'backup-maintenance',
		title: 'Backup and Maintenance',
		badge: 'OPERATIONS',
		description:
			'Maintain complete operational control with standard PostgreSQL backup tooling, zero-downtime health probes, and graceful process shutdown.',
		details: [
			'Standard pg_dump / wal-g database snapshots',
			'Graceful shutdown on SIGINT / SIGTERM signals',
			'Liveness (/healthz) & readiness (/readyz) monitoring'
		],
		accent: 'white',
		iconName: 'hard-drive-download'
	}
];

export const selfHostedComposeSnippet = `services:
  riralicense-api:
    build: ./server
    container_name: riralicense-api
    restart: unless-stopped
    ports:
      - "7819:7819"
    environment:
      APP_PORT: "7819"
      APP_ENV: "production"
      DATABASE_URL: "postgres://rira:secret@postgres:5432/riralicense?sslmode=disable"
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:7819/healthz"]
      interval: 15s
      timeout: 5s
      retries: 3

  postgres:
    image: postgres:17-alpine
    container_name: riralicense-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: "rira"
      POSTGRES_PASSWORD: "secret"
      POSTGRES_DB: "riralicense"
    volumes:
      - riralicense_pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U rira -d riralicense"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  riralicense_pgdata:`;

export const selfHostedEnvSnippet = `# RiraLicense Community Edition — Environment Configuration
APP_PORT=7819
APP_ENV=production

# Self-Managed PostgreSQL Connection (v1.0 Licensing Engine)
DATABASE_URL=postgres://rira:secret@localhost:5432/riralicense?sslmode=disable

# Routine PostgreSQL Backup Example
# pg_dump -Fc --no-acl --no-owner $DATABASE_URL > riralicense_$(date +%F).dump`;

export const heroDashboardSummary = {
	instanceName: 'self-hosted-node-01',
	activeLicenses: '1,480',
	totalProducts: '6',
	activeActivations: '3,912',
	validationStatus: '99.99% OK',
	productInventory: [
		{
			id: 'prod-1',
			name: 'RiraStudio Desktop',
			platform: 'Desktop • Node-Locked',
			licenseModel: 'Perpetual + Trial',
			activeKeys: 840,
			activations: '2,190 devices',
			status: 'ACTIVE' as LicenseStatus
		},
		{
			id: 'prod-2',
			name: 'RiraCore SaaS Control',
			platform: 'SaaS • Entitlements',
			licenseModel: 'Subscription',
			activeKeys: 410,
			activations: '1,280 seats',
			status: 'ACTIVE' as LicenseStatus
		},
		{
			id: 'prod-3',
			name: 'RiraBuild API Daemon',
			platform: 'API & Backend Service',
			licenseModel: 'Subscription',
			activeKeys: 230,
			activations: '442 nodes',
			status: 'TRIAL' as LicenseStatus
		}
	]
};

export const recentLicenseEvents: LicenseEventItem[] = [
	{
		id: 'evt-1',
		timestamp: 'Just now',
		eventType: 'license.validated',
		description: 'Offline signature & node-locked fingerprint verified (Linux x86_64)',
		key: 'RIRA-DSK-9482-X7K1',
		product: 'RiraStudio Desktop',
		status: 'VALID'
	},
	{
		id: 'evt-2',
		timestamp: '3m ago',
		eventType: 'device.activated',
		description: 'Bound device slot 2/2 (Fingerprint sha256:9f8a..21c)',
		key: 'RIRA-DSK-6190-L2N5',
		product: 'RiraStudio Desktop',
		status: 'ACTIVATED'
	},
	{
		id: 'evt-3',
		timestamp: '14m ago',
		eventType: 'audit.logged',
		description: 'Updated feature entitlements: [pro_export, api_access]',
		key: 'RIRA-SAS-3319-M4P8',
		product: 'RiraCore SaaS Control',
		status: 'AUDIT'
	}
];
