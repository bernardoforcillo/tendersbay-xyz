import { useTranslation } from 'react-i18next';
import { Eyebrow, Reveal } from '~/features/landing/components/atoms';

export function AudienceSection() {
  const { t } = useTranslation();
  return (
    <section
      id="audience"
      aria-labelledby="audience-title"
      className="relative scroll-mt-24 overflow-hidden bg-cream-50 py-28 md:py-32"
    >
      <div
        aria-hidden="true"
        className="pointer-events-none absolute left-1/2 top-1/2 h-[420px] w-[720px] -translate-x-1/2 -translate-y-1/2"
        style={{
          background: 'radial-gradient(50% 50% at 50% 50%, rgba(13,148,136,0.10), transparent 70%)',
        }}
      />
      <div className="relative mx-auto flex max-w-3xl flex-col items-center px-6 text-center">
        <Reveal className="flex flex-col items-center">
          <Eyebrow icon="check">{t('landing.audience.eyebrow')}</Eyebrow>
          <h2
            id="audience-title"
            className="mt-7 font-display text-[2.5rem] leading-[1.02] tracking-[-0.01em] text-ink-900 md:text-[3.4rem]"
          >
            {t('landing.audience.title')}
          </h2>
          <span aria-hidden="true" className="mt-8 h-px w-16 bg-brand-400" />
          <p className="mt-8 max-w-[46ch] text-lg leading-relaxed text-ink-600 md:text-xl">
            {t('landing.audience.body')}
          </p>
        </Reveal>
      </div>
    </section>
  );
}
