import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
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

const AGENTS = [
  {
    icon: 'search' as IconName,
    labelKey: 'landing.agentLabels.scout',
    statKey: 'landing.agentLabels.scoutStat',
  },
  {
    icon: 'document' as IconName,
    labelKey: 'landing.agentLabels.docs',
    statKey: 'landing.agentLabels.docsStat',
  },
  {
    icon: 'trophy' as IconName,
    labelKey: 'landing.agentLabels.strategy',
    statKey: 'landing.agentLabels.strategyStat',
  },
] as const satisfies ReadonlyArray<{ icon: IconName; labelKey: string; statKey: string }>;

function AgentChip({
  icon,
  label,
  status,
  className,
  float,
  delay = 0,
}: {
  icon: IconName;
  label: string;
  status: string;
  className?: string;
  float: boolean;
  delay?: number;
}) {
  return (
    <motion.div
      className={`absolute z-10 w-40 rounded-2xl border border-cream-300/80 bg-white/90 p-3 shadow-xl shadow-ink-900/10 backdrop-blur-sm ${className ?? ''}`}
      animate={float ? { y: [0, -7, 0] } : undefined}
      transition={float ? { duration: 5, repeat: Infinity, ease: 'easeInOut', delay } : undefined}
    >
      <div className="flex items-center gap-2">
        <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-brand-50 text-[15px] text-brand-600">
          <Icon name={icon} />
        </span>
        <span className="text-[13px] font-bold text-ink-900">{label}</span>
      </div>
      {status ? <p className="mt-1.5 text-[11px] font-semibold text-brand-600">{status}</p> : null}
    </motion.div>
  );
}

export function Hero() {
  const { t } = useTranslation();
  const { tender } = useRotatingTenders(SAMPLE_TENDERS);
  const reduce = useReducedMotion();
  const trust = t('landing.hero.trust', { returnObjects: true }) as string[];

  const container = reduce
    ? {}
    : {
        initial: 'hidden' as const,
        animate: 'show' as const,
        variants: {
          hidden: {},
          show: { transition: { staggerChildren: 0.09, delayChildren: 0.04 } },
        },
      };
  const item = reduce
    ? {}
    : {
        variants: {
          hidden: { opacity: 0, y: 18 },
          show: {
            opacity: 1,
            y: 0,
            transition: { duration: 0.6, ease: [0.22, 1, 0.36, 1] as const },
          },
        },
      };

  return (
    <section
      id="top"
      className="relative flex min-h-[88vh] items-center overflow-hidden bg-cream-100"
      aria-labelledby="hero-title"
    >
      {/* layered depth: aurora glow + second warm glow + fine grain */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0"
        style={{
          background:
            'radial-gradient(55% 70% at 92% -8%, rgba(13,148,136,0.18), transparent 60%), radial-gradient(45% 55% at 0% 108%, rgba(15,118,110,0.10), transparent 60%)',
        }}
      />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 opacity-[0.4]"
        style={{
          backgroundImage: 'radial-gradient(rgba(19,50,44,0.05) 1px, transparent 1px)',
          backgroundSize: '4px 4px',
        }}
      />

      <div className="relative mx-auto grid w-full max-w-6xl items-center gap-12 px-6 pt-28 pb-20 md:grid-cols-[1.05fr_0.95fr] md:pt-24 md:pb-16">
        <motion.div {...container}>
          <motion.div {...item}>
            <Eyebrow icon="sparkle">{t('landing.hero.eyebrow')}</Eyebrow>
          </motion.div>
          <motion.h1
            {...item}
            id="hero-title"
            className="mt-6 max-w-[15ch] font-display text-[2.8rem] leading-[1.02] tracking-[-0.01em] text-ink-900 md:text-[3.9rem]"
          >
            {t('landing.hero.titleLead')}{' '}
            <span className="bg-gradient-to-r from-brand-600 to-brand-700 bg-clip-text text-transparent">
              {t('landing.hero.titleHighlight')}
            </span>
          </motion.h1>
          <motion.p {...item} className="mt-6 max-w-[46ch] text-lg leading-relaxed text-ink-600">
            {t('landing.hero.subtitle')}
          </motion.p>
          <motion.div {...item} className="mt-9 flex flex-wrap items-center gap-5">
            <Button href="#agents" variant="primary">
              {t('landing.hero.ctaPrimary')}
              <Icon name="arrow-right" className="text-[18px]" />
            </Button>
            <Button href="#vision" variant="text">
              {t('landing.hero.ctaSecondary')}
            </Button>
          </motion.div>
          <motion.ul
            {...item}
            className="mt-8 flex flex-wrap items-center gap-x-3 gap-y-2 text-sm font-semibold text-ink-500"
          >
            {trust.map((entry, i) => (
              <li key={entry} className="flex items-center gap-3">
                {i > 0 ? (
                  <span aria-hidden="true" className="h-1 w-1 rounded-full bg-cream-400" />
                ) : null}
                {entry}
              </li>
            ))}
          </motion.ul>
        </motion.div>

        {/* agent constellation — visible on every breakpoint */}
        <motion.div
          aria-hidden="true"
          className="relative mx-auto h-[360px] w-[340px] shrink-0"
          initial={reduce ? undefined : { opacity: 0, scale: 0.94 }}
          animate={reduce ? undefined : { opacity: 1, scale: 1 }}
          transition={reduce ? undefined : { duration: 0.7, delay: 0.15, ease: [0.22, 1, 0.36, 1] }}
        >
          {/* halo */}
          <motion.div
            className="absolute left-1/2 top-1/2 h-56 w-56 -translate-x-1/2 -translate-y-1/2 rounded-full"
            style={{
              background:
                'radial-gradient(circle, rgba(13,148,136,0.22) 0%, rgba(13,148,136,0) 70%)',
            }}
            animate={reduce ? undefined : { scale: [1, 1.08, 1], opacity: [0.85, 1, 0.85] }}
            transition={reduce ? undefined : { duration: 6, repeat: Infinity, ease: 'easeInOut' }}
          />
          {/* dashed ring */}
          <motion.div
            className="absolute left-1/2 top-1/2 h-64 w-64 -translate-x-1/2 -translate-y-1/2 rounded-full border border-dashed border-brand-300/60"
            animate={reduce ? undefined : { rotate: 360 }}
            transition={reduce ? undefined : { duration: 60, repeat: Infinity, ease: 'linear' }}
          />
          {/* connectors */}
          <svg className="absolute inset-0 h-full w-full" viewBox="0 0 340 360" fill="none">
            <title>agent connections</title>
            <motion.g
              stroke="#9bcabf"
              strokeWidth="1.5"
              strokeDasharray="4 6"
              animate={reduce ? undefined : { strokeDashoffset: [0, -20] }}
              transition={reduce ? undefined : { duration: 1.6, repeat: Infinity, ease: 'linear' }}
            >
              <line x1="80" y1="48" x2="170" y2="180" />
              <line x1="262" y1="64" x2="170" y2="180" />
              <line x1="170" y1="312" x2="170" y2="180" />
            </motion.g>
          </svg>

          <AgentChip
            className="left-0 top-0"
            icon={AGENTS[0].icon}
            label={t(AGENTS[0].labelKey)}
            status={t(AGENTS[0].statKey, { count: tender.scoutCount })}
            float={!reduce}
            delay={0}
          />
          <AgentChip
            className="right-0 top-6"
            icon={AGENTS[1].icon}
            label={t(AGENTS[1].labelKey)}
            status={t(AGENTS[1].statKey)}
            float={!reduce}
            delay={0.8}
          />
          <AgentChip
            className="bottom-0 left-1/2 -translate-x-1/2"
            icon={AGENTS[2].icon}
            label={t(AGENTS[2].labelKey)}
            status={t(AGENTS[2].statKey)}
            float={!reduce}
            delay={1.6}
          />
          <div className="absolute left-1/2 top-1/2 z-20 -translate-x-1/2 -translate-y-1/2">
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
        </motion.div>
      </div>

      {/* scroll cue */}
      <motion.div
        aria-hidden="true"
        className="-translate-x-1/2 absolute bottom-6 left-1/2 hidden text-ink-400 md:block"
        animate={reduce ? undefined : { y: [0, 6, 0] }}
        transition={reduce ? undefined : { duration: 2, repeat: Infinity, ease: 'easeInOut' }}
      >
        <Icon name="chevron-down" className="text-[22px]" />
      </motion.div>
    </section>
  );
}
