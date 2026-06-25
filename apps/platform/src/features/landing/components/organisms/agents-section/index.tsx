import { useTranslation } from 'react-i18next';
import { Eyebrow, type IconName, Reveal } from '~/features/landing/components/atoms';
import { AgentStep } from '~/features/landing/components/molecules';

type Item = { title: string; body: string };
const ICONS: IconName[] = ['search', 'document', 'trophy'];

export function AgentsSection() {
  const { t } = useTranslation();
  const items = t('landing.agents.items', { returnObjects: true }) as Item[];

  return (
    <section
      id="agents"
      aria-labelledby="agents-title"
      className="relative scroll-mt-24 overflow-hidden bg-ink-900 py-24 text-ink-100 md:py-28"
    >
      {/* aurora top-edge bleeding down from the light Problem section */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-0 top-0 h-64"
        style={{
          background:
            'radial-gradient(60% 80% at 50% -10%, rgba(45,212,191,0.16), transparent 70%)',
        }}
      />
      <div className="relative mx-auto max-w-6xl px-6">
        <Reveal>
          <Eyebrow icon="sparkle" className="border-white/15 bg-white/5 text-brand-300">
            {t('landing.agents.eyebrow')}
          </Eyebrow>
          <h2
            id="agents-title"
            className="mt-5 max-w-[20ch] font-display text-[2rem] leading-[1.05] tracking-tight text-white md:text-[2.7rem]"
          >
            {t('landing.agents.title')}
          </h2>
        </Reveal>

        <div className="relative mt-14 grid gap-12 md:grid-cols-3 md:gap-8">
          {items.map((item, i) => (
            <Reveal key={item.title} delay={i * 0.1}>
              <AgentStep
                index={i + 1}
                icon={ICONS[i] ?? 'sparkle'}
                title={item.title}
                body={item.body}
              />
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}
