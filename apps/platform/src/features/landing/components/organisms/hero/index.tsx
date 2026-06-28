import { motion, useReducedMotion } from 'motion/react';
import { useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Icon } from '~/features/landing/components/atoms';
import { useParallax } from '~/features/landing/motion';
import { SAMPLE_TENDERS } from './sample-tenders';
import { TenderDeck } from './tender-deck';

export function Hero() {
  const { t } = useTranslation();
  const reduce = useReducedMotion();
  const trust = t('landing.hero.trust', { returnObjects: true }) as string[];
  const heroRef = useRef<HTMLElement>(null);
  const constellationY = useParallax(heroRef, 80);

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
      ref={heroRef}
      id="top"
      className="relative flex min-h-[88vh] items-center overflow-hidden bg-cream-100"
      aria-labelledby="hero-title"
    >
      <div className="relative mx-auto grid w-full max-w-6xl items-center gap-12 px-6 pt-28 pb-20 md:grid-cols-[1.05fr_0.95fr] md:pt-24 md:pb-16">
        <motion.div {...container}>
          <motion.h1
            {...item}
            id="hero-title"
            className="max-w-[15ch] font-display text-[2.8rem] leading-[1.02] tracking-[-0.01em] text-ink-900 md:text-[3.9rem]"
          >
            <span className="block">{t('landing.hero.titleLead')}</span>
            <span className="relative mt-2 inline-block bg-gradient-to-r from-brand-600 to-brand-700 bg-clip-text text-transparent">
              {t('landing.hero.titleHighlight')}
              <span
                aria-hidden="true"
                className="absolute -bottom-1 left-0 h-2 w-full rounded-full bg-brand-400/40"
              />
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
            className="mt-8 flex flex-wrap items-center gap-x-5 gap-y-2 text-sm font-semibold text-ink-700"
          >
            {trust.map((entry) => (
              <li key={entry} className="flex items-center gap-1.5">
                <Icon name="check" className="text-[15px] text-brand-600" />
                {entry}
              </li>
            ))}
          </motion.ul>
        </motion.div>

        {/* infinite tender deck — a "Tinder" swipe loop on every breakpoint */}
        <motion.div
          aria-hidden="true"
          className="relative mx-auto flex h-[400px] w-[340px] shrink-0 items-center justify-center"
          style={constellationY ? { y: constellationY } : undefined}
          initial={reduce ? undefined : { opacity: 0, scale: 0.94 }}
          animate={reduce ? undefined : { opacity: 1, scale: 1 }}
          transition={reduce ? undefined : { duration: 0.7, delay: 0.15, ease: [0.22, 1, 0.36, 1] }}
        >
          <TenderDeck tenders={SAMPLE_TENDERS} />
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
