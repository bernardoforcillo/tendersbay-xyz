import {
  AgentsSection,
  AudienceSection,
  CoverageSection,
  Hero,
  ProblemSection,
  SiteFooter,
  SiteHeader,
  VisionSection,
} from '~/features/landing/components/organisms';

export function LandingTemplate() {
  return (
    <div className="relative min-h-screen bg-cream-100">
      <SiteHeader />
      <main>
        <Hero />
        <ProblemSection />
        <AgentsSection />
        <AudienceSection />
        <CoverageSection />
        <VisionSection />
      </main>
      <SiteFooter />
    </div>
  );
}
