export interface FAQItem {
	id: string;
	question: string;
	answer: string;
	category: string;
}

export const faqItems: FAQItem[] = [
	{
		id: 'faq-1',
		category: 'OPEN SOURCE',
		question: 'Is RiraLicense open source?',
		answer:
			'Yes. RiraLicense is an open-source software licensing infrastructure platform released under the GNU Affero General Public License v3.0 (AGPL-3.0). You can inspect the complete source code, audit the licensing engine, and contribute on GitHub.'
	},
	{
		id: 'faq-2',
		category: 'SELF-HOSTING',
		question: 'Can I self-host it?',
		answer:
			'Yes. RiraLicense Community Edition is built specifically for independent self-hosting using Docker Compose and PostgreSQL. You run the licensing server on your own infrastructure, retain full ownership of your database and signing keys, and operate without any dependency on external RiraLicense-hosted services.'
	},
	{
		id: 'faq-3',
		category: 'LICENSE MODELS',
		question: 'What types of licenses are supported?',
		answer:
			'The RiraLicense core licensing domain is designed to support perpetual licenses, time-bound subscription licenses, trial licenses, node-locked device activations, and feature-based entitlements across desktop, SaaS, and API products.'
	},
	{
		id: 'faq-4',
		category: 'INTEGRATION',
		question: 'Can I integrate it with my existing application?',
		answer:
			'Yes. RiraLicense exposes clean HTTP REST API contracts that can be called from any programming language, desktop client, SaaS backend, or API gateway. Official client SDKs are also planned for the v1.0 release to simplify integration even further.'
	},
	{
		id: 'faq-5',
		category: 'OFFLINE VERIFICATION',
		question: 'Does it support offline validation?',
		answer:
			'Offline license verification using asymmetric digital signatures (such as Ed25519) is a core capability of the RiraLicense v1.0 architecture. By embedding only your public verification key inside your application, desktop and on-premise software can verify signed license grants without constant internet connectivity.'
	},
	{
		id: 'faq-6',
		category: 'DOCUMENTATION',
		question: 'Where can I find documentation?',
		answer:
			'Setup instructions, environment variable references, architecture guides, and contribution rules are available directly in the GitHub repository README and documentation files, alongside the deployment and API examples on this page.'
	}
];
