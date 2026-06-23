import { useTranslation } from 'react-i18next';
import { Eyebrow, Reveal } from '~/features/landing/components/atoms';

export function VisionSection() {
  const { t } = useTranslation();
  return (
    <section
      id="vision"
      aria-labelledby="vision-title"
      className="scroll-mt-24 bg-ink-900 py-24 text-ink-100"
    >
      <div className="mx-auto max-w-3xl px-6 text-center">
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
          <p className="mt-8 inline-flex items-center gap-2 rounded-full border border-white/15 bg-white/5 px-4 py-2 text-sm font-semibold text-brand-300">
            {t('landing.vision.note')}
          </p>
        </Reveal>
      </div>
    </section>
  );
}
