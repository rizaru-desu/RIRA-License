<script lang="ts">
	import Navbar from '$lib/components/layout/Navbar.svelte';
	import Footer from '$lib/components/layout/Footer.svelte';
	import Hero from '$lib/components/landing/Hero.svelte';
	import PlatformSupport from '$lib/components/landing/PlatformSupport.svelte';
	import Features from '$lib/components/landing/Features.svelte';
	import HowItWorks from '$lib/components/landing/HowItWorks.svelte';
	import DeveloperExperience from '$lib/components/landing/DeveloperExperience.svelte';
	import DeploymentOptions from '$lib/components/landing/DeploymentOptions.svelte';
	import Community from '$lib/components/landing/Community.svelte';
	import FAQ from '$lib/components/landing/FAQ.svelte';
	import FinalCTA from '$lib/components/landing/FinalCTA.svelte';
	import { siteConfig } from '$lib/data/navigation';
</script>

<svelte:head>
	<title>{siteConfig.seoTitle}</title>
	<meta name="description" content={siteConfig.seoDescription} />
	<!-- Open Graph Metadata -->
	<meta property="og:type" content="website" />
	<meta property="og:site_name" content="RiraLicense" />
	<meta property="og:title" content={siteConfig.seoTitle} />
	<meta property="og:description" content={siteConfig.seoDescription} />
	<!-- Twitter Card Metadata -->
	<meta name="twitter:card" content="summary_large_image" />
	<meta name="twitter:title" content={siteConfig.seoTitle} />
	<meta name="twitter:description" content={siteConfig.seoDescription} />
</svelte:head>

<!-- 1. Navigation -->
<Navbar />

<main class="flex-1">
	<!-- 2. Hero -->
	<Hero />

	<!-- 3. Supported Software -->
	<PlatformSupport />

	<!-- 4. Core Features -->
	<Features />

	<!-- 5. How It Works -->
	<HowItWorks />

	<!-- 6. Developer Experience -->
	<DeveloperExperience />

	<!-- 7. Self-Hosted Deployment -->
	<DeploymentOptions />

	<!-- 8. Open Source Community -->
	<Community />

	<!-- 9. FAQ -->
	<FAQ />

	<!-- 10. Final CTA -->
	<FinalCTA />
</main>

<!-- 11. Footer -->
<Footer />
