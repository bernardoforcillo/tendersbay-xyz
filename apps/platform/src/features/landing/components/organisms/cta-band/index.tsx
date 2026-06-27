import { motion } from 'motion/react';
import { useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Icon } from '~/features/landing/components/atoms';
import { useKineticBlock } from '~/features/landing/motion';

export function CtaBand() {
  const { t } = useTranslation();
  const sectionRef = useRef<HTMLElement>(null);
  const kinetic = useKineticBlock(sectionRef);

  return (
    <section ref={sectionRef} aria-labelledby="cta-title" className="px-6 py-16 md:py-20">
      <motion.div
        style={kinetic}
        className="relative mx-auto max-w-6xl overflow-hidden rounded-3xl bg-brand-700 px-8 py-16 text-center shadow-soft-lg md:px-16 md:py-20"
      >
        <div className="relative mx-auto flex max-w-2xl flex-col items-center">
          <span className="inline-flex items-center gap-2 rounded-full border border-white/25 bg-white/10 px-3 py-1.5 font-mono text-[11px] font-semibold uppercase tracking-[0.16em] text-white">
            <Icon name="sparkle" className="text-[14px]" />
            {t('landing.cta.eyebrow')}
          </span>
          <h2
            id="cta-title"
            className="mt-5 font-display text-[2rem] leading-[1.05] tracking-tight text-white md:text-[2.7rem]"
          >
            {t('landing.cta.title')}
          </h2>
          <p className="mt-4 text-[15px] leading-relaxed text-brand-50 md:text-base">
            {t('landing.cta.body')}
          </p>
          <Button href="#top" variant="invert" className="mt-8">
            {t('landing.cta.button')}
          </Button>
        </div>
      </motion.div>
    </section>
  );
}
