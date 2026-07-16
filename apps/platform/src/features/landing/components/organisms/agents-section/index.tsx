import { useInView } from 'motion/react';
import { usePostHog } from 'posthog-js/react';
import { useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Icon, type IconName, Reveal } from '~/features/landing/components/atoms';

type Item = { time: string; title: string; body: string };
const ICONS: IconName[] = ['search', 'document', 'trophy'];

export function AgentsSection() {
  const { t } = useTranslation();
  const items = t('landing.agents.items', { returnObjects: true }) as Item[];

  // Measure whether the open-loop hook actually pulls readers in: fire once when
  // the section scrolls into view, so we can A/B this framing against the CTA
  // conversion downstream. Consent-gating is automatic (opt-out by default).
  const posthog = usePostHog();
  const sectionRef = useRef<HTMLElement>(null);
  const inView = useInView(sectionRef, { once: true, amount: 0.4 });
  useEffect(() => {
    if (inView) {
      posthog?.capture('agents_section_viewed', { location: 'agents' });
    }
  }, [inView, posthog]);

  return (
    <section
      ref={sectionRef}
      id="agents"
      aria-labelledby="agents-title"
      className="scroll-mt-24 bg-brand-700 py-24 text-white md:py-28"
    >
      <div className="mx-auto max-w-6xl px-6">
        <Reveal>
          <h2
            id="agents-title"
            className="max-w-[24ch] font-display text-[2rem] leading-[1.05] tracking-tight text-white md:text-[2.7rem]"
          >
            {t('landing.agents.title')}
          </h2>
          {/* The tools-vs-agents wedge: a category redefinition, not a feature list. */}
          <p className="mt-5 max-w-[60ch] text-lg leading-relaxed text-brand-50">
            {t('landing.agents.lead')}
          </p>
        </Reveal>

        {/* One team, working in parallel — no sequential numbering. */}
        <div className="mt-14 grid gap-12 md:grid-cols-3 md:gap-8">
          {items.map((item, i) => (
            <Reveal key={item.title} delay={i * 0.08}>
              {/* Icon + timestamp: the three read as a 02:14 → 05:30 → 07:00 overnight timeline. */}
              <div className="flex items-center gap-3">
                <span className="inline-flex h-12 w-12 items-center justify-center rounded-2xl bg-white/10 text-[21px] text-white ring-1 ring-white/20">
                  <Icon name={ICONS[i] ?? 'sparkle'} />
                </span>
                <time className="font-mono text-sm font-semibold tabular-nums tracking-[0.2em] text-brand-200">
                  {item.time}
                </time>
              </div>
              <h3 className="mt-5 font-display text-xl text-white">{item.title}</h3>
              <p className="mt-2.5 text-[15px] leading-relaxed text-brand-50">{item.body}</p>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}
