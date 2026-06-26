import { motion, useReducedMotion } from 'motion/react';
import { useTranslation } from 'react-i18next';
import { Eyebrow, Reveal } from '~/features/landing/components/atoms';

export function VisionSection() {
  const { t } = useTranslation();
  const reduce = useReducedMotion();
  return (
    <section
      id="vision"
      aria-labelledby="vision-title"
      className="relative scroll-mt-24 overflow-hidden bg-ink-900 py-24 text-ink-100 md:py-28"
    >
      {/* aurora top-edge from the light Audience above */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-0 top-0 h-64"
        style={{
          background:
            'radial-gradient(60% 80% at 50% -10%, rgba(45,212,191,0.14), transparent 70%)',
        }}
      />
      {/* closing aurora — bookends back to the hero */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-0 bottom-0 h-72"
        style={{
          background:
            'radial-gradient(55% 80% at 50% 120%, rgba(13,148,136,0.18), transparent 70%)',
        }}
      />
      <div className="relative mx-auto max-w-3xl px-6 text-center">
        <Reveal>
          <Eyebrow icon="sparkle" className="mx-auto border-white/15 bg-white/5 text-brand-300">
            {t('landing.vision.eyebrow')}
          </Eyebrow>
          <h2
            id="vision-title"
            className="mt-5 font-display text-[2rem] leading-[1.05] tracking-tight text-white md:text-[2.7rem]"
          >
            {t('landing.vision.title')}
          </h2>
          <p className="mx-auto mt-5 max-w-[58ch] text-lg leading-relaxed text-ink-200">
            {t('landing.vision.body')}
          </p>
          <p className="mt-9 inline-flex items-center gap-2.5 rounded-full border border-white/10 bg-white/5 px-4 py-2 font-mono text-xs font-semibold uppercase tracking-[0.14em] text-brand-300">
            <span className="relative flex h-2 w-2">
              {reduce ? null : (
                <motion.span
                  className="absolute inline-flex h-full w-full rounded-full bg-brand-400"
                  animate={{ scale: [1, 2.2], opacity: [0.6, 0] }}
                  transition={{ duration: 1.8, repeat: Infinity, ease: 'easeOut' }}
                />
              )}
              <span className="relative inline-flex h-2 w-2 rounded-full bg-brand-400" />
            </span>
            {t('landing.vision.note')}
          </p>
        </Reveal>
      </div>
    </section>
  );
}
