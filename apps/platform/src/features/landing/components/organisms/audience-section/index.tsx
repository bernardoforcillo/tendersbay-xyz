import { useTranslation } from 'react-i18next';
import { Eyebrow, Reveal } from '~/features/landing/components/atoms';

export function AudienceSection() {
  const { t } = useTranslation();
  return (
    <section
      id="audience"
      aria-labelledby="audience-title"
      className="scroll-mt-24 bg-cream-100 py-20"
    >
      <div className="mx-auto max-w-3xl px-6 text-center">
        <Reveal>
          <Eyebrow icon="check" className="mx-auto">
            {t('landing.audience.eyebrow')}
          </Eyebrow>
          <h2
            id="audience-title"
            className="mt-4 text-3xl font-extrabold tracking-tight text-ink-900 md:text-4xl"
          >
            {t('landing.audience.title')}
          </h2>
          <p className="mx-auto mt-5 max-w-[52ch] text-lg leading-relaxed text-ink-600">
            {t('landing.audience.body')}
          </p>
        </Reveal>
      </div>
    </section>
  );
}
