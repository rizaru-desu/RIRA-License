export interface NavLink {
	label: string;
	href: string;
	external?: boolean;
}

export interface CommunityCardItem {
	id: string;
	title: string;
	category: string;
	description: string;
	href: string;
	ctaLabel: string;
	external?: boolean;
	accent: 'yellow' | 'cobalt' | 'coral' | 'white';
	iconName: 'github' | 'git-pull-request' | 'bug' | 'messages-square' | 'map';
}

export interface RoadmapMilestoneItem {
	id: string;
	module: string;
	title: string;
	description: string;
	status: 'AVAILABLE' | 'IN PROGRESS' | 'PLANNED v1.0';
	accent: 'yellow' | 'cobalt' | 'coral' | 'white';
}

export const siteConfig = {
	name: 'RiraLicense',
	edition: 'Community Edition',
	positioning: 'Open-source Software Licensing Infrastructure.',
	tagline: 'Your Software. Your Licenses. Your Control.',
	supportingMessage:
		'An open-source platform to create, manage, validate, and protect software licenses across desktop applications, SaaS products, and APIs.',
	seoTitle: 'RiraLicense — Open-Source Software Licensing Infrastructure',
	seoDescription:
		'An open-source, self-hosted platform to create, manage, validate, and protect software licenses across desktop applications, SaaS products, and APIs.',
	coreLicense: 'GNU AGPL-3.0',
	githubUrl: 'https://github.com/rizaru-desu/RIRA-License',
	githubReadmeUrl: 'https://github.com/rizaru-desu/RIRA-License#readme',
	githubLicenseUrl: 'https://github.com/rizaru-desu/RIRA-License/blob/main/LICENSE',
	githubNoticeUrl: 'https://github.com/rizaru-desu/RIRA-License/blob/main/NOTICE',
	githubContributingUrl: 'https://github.com/rizaru-desu/RIRA-License/blob/main/CODE_OF_CONDUCT.md',
	githubIssuesUrl: 'https://github.com/rizaru-desu/RIRA-License/issues',
	githubDiscussionsUrl: 'https://github.com/rizaru-desu/RIRA-License/discussions',
	getStartedHref: '#deployment',
	documentationHref: '#deployment',
	apiReferenceHref: '#developer-experience'
} as const;

export const primaryNavItems: NavLink[] = [
	{ label: 'Features', href: '#features' },
	{ label: 'Documentation', href: '#deployment' },
	{ label: 'API Reference', href: '#developer-experience' },
	{ label: 'GitHub', href: siteConfig.githubUrl, external: true }
];

export const communityCards: CommunityCardItem[] = [
	{
		id: 'github-repository',
		title: 'GitHub Repository',
		category: 'SOURCE CODE',
		description:
			'Explore the Go + chi backend server and Svelte 5 web console. Inspect the code, star the project, or fork it for self-hosted deployments.',
		href: siteConfig.githubUrl,
		ctaLabel: 'Browse Repository',
		external: true,
		accent: 'yellow',
		iconName: 'github'
	},
	{
		id: 'contributions',
		title: 'Contributions',
		category: 'PULL REQUESTS',
		description:
			'Contribute core licensing domain logic, database adapters, documentation, or client SDKs following our clean Modular Monolith architecture.',
		href: siteConfig.githubContributingUrl,
		ctaLabel: 'Contribution Guidelines',
		external: true,
		accent: 'cobalt',
		iconName: 'git-pull-request'
	},
	{
		id: 'issue-reporting',
		title: 'Issue Reporting',
		category: 'BUGS & SECURITY',
		description:
			'Found an edge case in license verification, Docker deployment, or API handlers? Open a reproducible issue directly on GitHub.',
		href: siteConfig.githubIssuesUrl,
		ctaLabel: 'Report an Issue',
		external: true,
		accent: 'coral',
		iconName: 'bug'
	},
	{
		id: 'discussions',
		title: 'Discussions',
		category: 'COMMUNITY RFCs',
		description:
			'Discuss self-hosting topologies, offline signature workflows, and API design proposals with maintainers and fellow developers.',
		href: siteConfig.githubDiscussionsUrl,
		ctaLabel: 'Join Discussions',
		external: true,
		accent: 'white',
		iconName: 'messages-square'
	}
];

export const communityRoadmap: RoadmapMilestoneItem[] = [
	{
		id: 'rm-1',
		module: 'SERVER FOUNDATION',
		title: 'Go + chi HTTP Server & Health Probes',
		description:
			'Modular handler -> service -> repo architecture, structured middleware, graceful shutdown, and /healthz & /readyz endpoints.',
		status: 'AVAILABLE',
		accent: 'yellow'
	},
	{
		id: 'rm-2',
		module: 'WEB INTERFACE',
		title: 'Svelte 5 Control Plane UI & Design System',
		description:
			'Modern Neo-Brutalist web interface and reusable component architecture built with Svelte 5 Runes, TypeScript, and Tailwind CSS 4.',
		status: 'AVAILABLE',
		accent: 'yellow'
	},
	{
		id: 'rm-3',
		module: 'LICENSING ENGINE',
		title: 'Core License Lifecycle, Entitlements & Device Activation',
		description:
			'PostgreSQL schema migrations, key generation, perpetual/subscription/trial policies, node-locked activations, and audit logs.',
		status: 'IN PROGRESS',
		accent: 'cobalt'
	},
	{
		id: 'rm-4',
		module: 'CRYPTOGRAPHY & SDKs',
		title: 'Signed Offline Verification, Webhooks & Official SDKs',
		description:
			'Ed25519 digital signature verification for offline grants, lifecycle webhook dispatchers, and initial official client SDK release.',
		status: 'PLANNED v1.0',
		accent: 'coral'
	}
];

export const footerNavigation = {
	documentation: [
		{ label: 'Self-Hosted Deployment', href: '#deployment' },
		{ label: 'How It Works', href: '#how-it-works' },
		{ label: 'Supported Software', href: '#supported-software' },
		{ label: 'FAQ', href: '#faq' }
	],
	apiReference: [
		{ label: 'REST API Examples', href: '#developer-experience' },
		{ label: 'SDK Integration Preview', href: '#developer-experience' },
		{ label: 'Core Features', href: '#features' },
		{ label: 'Health & Readiness Probes', href: '#deployment' }
	],
	community: [
		{ label: 'GitHub Repository', href: siteConfig.githubUrl, external: true },
		{ label: 'Open Source Roadmap', href: '#community' },
		{ label: 'Report an Issue', href: siteConfig.githubIssuesUrl, external: true },
		{ label: 'Discussions', href: siteConfig.githubDiscussionsUrl, external: true }
	],
	licenseInfo: [
		{ label: 'GNU AGPL-3.0 License', href: siteConfig.githubLicenseUrl, external: true },
		{ label: 'Copyright & Trademark Notice', href: siteConfig.githubNoticeUrl, external: true },
		{ label: 'Code of Conduct', href: siteConfig.githubContributingUrl, external: true }
	]
} as const;
