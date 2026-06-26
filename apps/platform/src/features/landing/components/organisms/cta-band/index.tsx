import { useTranslation } from 'react-i18next';
import { Button, Icon } from '~/features/landing/components/atoms';

export function CtaBand() {
  const { t } = useTranslation();

  return (
    <section aria-labelledby="cta-title" className="px-6 py-16 md:py-20">
      <div className="relative mx-auto max-w-6xl overflow-hidden rounded-3xl bg-ink-950 px-8 py-16 text-center shadow-soft-lg md:px-16 md:py-20">
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 bg-[radial-gradient(120%_120%_at_50%_-10%,rgba(45,212,191,0.25),transparent_60%)]"
        />
        <div className="relative mx-auto flex max-w-2xl flex-col items-center">
          <span className="inline-flex items-center gap-2 rounded-full border border-white/15 bg-white/5 px-3 py-1.5 font-mono text-[11px] font-semibold uppercase tracking-[0.16em] text-brand-300">
            <Icon name="sparkle" className="text-[14px]" />
            {t('landing.cta.eyebrow')}
          </span>
          <h2
            id="cta-title"
            className="mt-5 font-display text-[2rem] leading-[1.05] tracking-tight text-white md:text-[2.7rem]"
          >
            {t('landing.cta.title')}
          </h2>
          <p className="mt-4 text-[15px] leading-relaxed text-ink-200 md:text-base">
            {t('landing.cta.body')}
          </p>
          <Button href="#top" className="mt-8">
            {t('landing.cta.button')}
          </Button>
        </div>
      </div>
    </section>
  );
}
