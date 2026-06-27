import { useTranslation } from 'react-i18next';
import { Icon, type IconName, Reveal } from '~/features/landing/components/atoms';

type Item = { title: string; body: string };
const ICONS: IconName[] = ['search', 'document', 'trophy'];

export function AgentsSection() {
  const { t } = useTranslation();
  const items = t('landing.agents.items', { returnObjects: true }) as Item[];

  return (
    <section
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
        </Reveal>

        {/* One team, working in parallel — no sequential numbering. */}
        <div className="mt-14 grid gap-12 md:grid-cols-3 md:gap-8">
          {items.map((item, i) => (
            <Reveal key={item.title} delay={i * 0.08}>
              <span className="inline-flex h-12 w-12 items-center justify-center rounded-2xl bg-white/10 text-[21px] text-white ring-1 ring-white/20">
                <Icon name={ICONS[i] ?? 'sparkle'} />
              </span>
              <h3 className="mt-5 font-display text-xl text-white">{item.title}</h3>
              <p className="mt-2.5 text-[15px] leading-relaxed text-brand-50">{item.body}</p>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}
