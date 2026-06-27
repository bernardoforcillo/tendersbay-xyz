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
      className="scroll-mt-24 bg-cream-100 py-24 md:py-28"
    >
      <div className="mx-auto max-w-3xl px-6 text-center">
        <Reveal>
          <Eyebrow icon="sparkle" className="mx-auto">
            {t('landing.vision.eyebrow')}
          </Eyebrow>
          <h2
            id="vision-title"
            className="mt-5 font-display text-[2rem] leading-[1.05] tracking-tight text-ink-900 md:text-[2.7rem]"
          >
            {t('landing.vision.title')}
          </h2>
          <p className="mx-auto mt-5 max-w-[58ch] text-lg leading-relaxed text-ink-600">
            {t('landing.vision.body')}
          </p>
          <p className="mt-9 inline-flex items-center gap-2.5 rounded-full border border-brand-200 bg-brand-50 px-4 py-2 font-mono text-xs font-semibold uppercase tracking-[0.14em] text-brand-700">
            <span className="relative flex h-2 w-2">
              {reduce ? null : (
                <motion.span
                  className="absolute inline-flex h-full w-full rounded-full bg-brand-400"
                  animate={{ scale: [1, 2.2], opacity: [0.6, 0] }}
                  transition={{ duration: 1.8, repeat: Infinity, ease: 'easeOut' }}
                />
              )}
              <span className="relative inline-flex h-2 w-2 rounded-full bg-brand-500" />
            </span>
            {t('landing.vision.note')}
          </p>
        </Reveal>
      </div>
    </section>
  );
}
