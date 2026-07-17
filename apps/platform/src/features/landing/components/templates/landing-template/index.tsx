import {
  AgentsSection,
  AssuranceSection,
  AudienceSection,
  CoverageSection,
  CtaBand,
  Hero,
  ProblemSection,
  ProofStrip,
  SearchDock,
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
        <ProofStrip />
        <ProblemSection />
        <AgentsSection />
        <AudienceSection />
        <AssuranceSection />
        <CoverageSection />
        <VisionSection />
      </main>
      <CtaBand />
      <SiteFooter />
      <SearchDock />
    </div>
  );
}
