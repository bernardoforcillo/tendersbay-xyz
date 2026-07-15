import { cn } from '@tendersbay/components/core';
import { usePostHog } from 'posthog-js/react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';

/** The four teaser filters, in render order. Values map to `landing.search.filters.*`. */
const FILTER_KEYS = ['country', 'sector', 'deadline', 'value'] as const;

/**
 * A row of disabled, grayscale filter chips beneath the search dock — a
 * pre-launch teaser hinting at how you'll narrow tenders. Each chip stays
 * keyboard-focusable (`aria-disabled`, not `isDisabled`) with a no-op press,
 * matching the dock's coming-soon treatment. Rendered inside the dock's fading
 * container, so it hides near the footer with the bar.
 */
export function FilterChips() {
  const { t } = useTranslation();
  const posthog = usePostHog();

  const captureHover = (chip: (typeof FILTER_KEYS)[number]) =>
    posthog?.capture('landing_search_filter_hovered', { chip, location: 'search_dock' });

  return (
    <div className="flex flex-wrap items-center justify-center gap-2">
      {FILTER_KEYS.map((key) => (
        <Button
          key={key}
          type="button"
          aria-disabled="true"
          onPress={() => {}}
          onHoverStart={() => captureHover(key)}
          onFocus={() => captureHover(key)}
          className={cn(
            'cursor-default rounded-full border border-ink-200 bg-white/70 px-3 py-1 text-xs font-medium text-ink-500',
            'shadow-soft backdrop-blur grayscale outline-none',
            'data-[focus-visible]:ring-2 data-[focus-visible]:ring-ink-300',
            'data-[focus-visible]:ring-offset-2 data-[focus-visible]:ring-offset-cream-100',
          )}
        >
          {t(`landing.search.filters.${key}`)}
        </Button>
      ))}
    </div>
  );
}
