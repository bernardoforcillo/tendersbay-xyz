import { AnimatePresence, motion } from 'motion/react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Eyebrow,
  Icon,
  type IconName,
  TenderCard,
} from '~/features/landing/components/atoms';
import { SAMPLE_TENDERS } from './sample-tenders';
import { useRotatingTenders } from './use-rotating-tenders';

const AGENTS: Array<{ icon: IconName; labelKey: string; status: string }> = [
  { icon: 'search', labelKey: 'landing.agentLabels.scout', status: '' },
  { icon: 'document', labelKey: 'landing.agentLabels.docs', status: 'fascicolo' },
  { icon: 'trophy', labelKey: 'landing.agentLabels.strategy', status: 'offerta' },
];

function AgentChip({ icon, label, status }: { icon: IconName; label: string; status: string }) {
  return (
    <div className="w-44 rounded-2xl border border-cream-300 bg-white p-3 shadow-lg shadow-ink-900/5">
      <div className="flex items-center gap-2">
        <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-brand-50 text-[15px] text-brand-600">
          <Icon name={icon} />
        </span>
        <span className="text-[13px] font-bold text-ink-900">{label}</span>
      </div>
      {status ? <p className="mt-1.5 text-[11px] font-semibold text-brand-600">{status}</p> : null}
    </div>
  );
}

export function Hero() {
  const { t } = useTranslation();
  const { tender } = useRotatingTenders(SAMPLE_TENDERS);
  const trust = t('landing.hero.trust', { returnObjects: true }) as string[];

  return (
    <section
      id="top"
      className="relative overflow-hidden bg-cream-100"
      aria-labelledby="hero-title"
    >
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0"
        style={{
          background:
            'radial-gradient(60% 80% at 88% -10%, rgba(13, 148, 136, 0.15), transparent 60%)',
        }}
      />
      <div className="relative mx-auto grid max-w-6xl items-center gap-10 px-6 py-20 md:grid-cols-2">
        <div>
          <Eyebrow icon="sparkle">{t('landing.hero.eyebrow')}</Eyebrow>
          <h1
            id="hero-title"
            className="mt-5 max-w-[14ch] text-4xl font-extrabold leading-[1.03] tracking-tight text-ink-900 md:text-5xl"
          >
            {t('landing.hero.titleLead')}{' '}
            <span className="bg-gradient-to-r from-brand-600 to-brand-700 bg-clip-text text-transparent">
              {t('landing.hero.titleHighlight')}
            </span>
            .
          </h1>
          <p className="mt-5 max-w-[48ch] text-lg leading-relaxed text-ink-600">
            {t('landing.hero.subtitle')}
          </p>
          <div className="mt-8 flex flex-wrap items-center gap-5">
            <Button href="#agents" variant="primary">
              {t('landing.hero.ctaPrimary')}
              <Icon name="arrow-right" className="text-[18px]" />
            </Button>
            <Button href="#vision" variant="text">
              {t('landing.hero.ctaSecondary')}
            </Button>
          </div>
          <ul className="mt-7 flex flex-wrap items-center gap-3 text-sm font-semibold text-ink-500">
            {trust.map((item, i) => (
              <li key={item} className="flex items-center gap-3">
                {i > 0 ? (
                  <span aria-hidden="true" className="h-1 w-1 rounded-full bg-cream-400" />
                ) : null}
                {item}
              </li>
            ))}
          </ul>
        </div>

        <div className="relative mx-auto hidden h-[360px] w-[360px] md:block" aria-hidden="true">
          <AgentChip
            icon={AGENTS[0].icon}
            label={t(AGENTS[0].labelKey)}
            status={t('landing.agentLabels.scoutStat', { count: tender.scoutCount })}
          />
          <div className="absolute right-0 top-5">
            <AgentChip
              icon={AGENTS[1].icon}
              label={t(AGENTS[1].labelKey)}
              status={AGENTS[1].status}
            />
          </div>
          <div className="absolute bottom-0 left-1/2 -translate-x-1/2">
            <AgentChip
              icon={AGENTS[2].icon}
              label={t(AGENTS[2].labelKey)}
              status={AGENTS[2].status}
            />
          </div>
          <div className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2">
            <AnimatePresence mode="wait">
              <motion.div
                key={tender.id}
                initial={{ opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -8 }}
                transition={{ duration: 0.4 }}
              >
                <TenderCard tender={tender} />
              </motion.div>
            </AnimatePresence>
          </div>
        </div>
      </div>
    </section>
  );
}
