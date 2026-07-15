import { cn } from '@tendersbay/components/core';
import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
import { usePostHog } from 'posthog-js/react';
import { useRef } from 'react';
import { Button, Tooltip, TooltipTrigger } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { Icon } from '~/features/landing/components/atoms';
import { FilterChips } from './filter-chips';
import { useHideNearFooter } from './use-hide-near-footer';
import { useRotatingPlaceholder } from './use-rotating-placeholder';

export function SearchDock() {
  const { t } = useTranslation();
  const posthog = usePostHog();
  const reduce = useReducedMotion();
  const hidden = useHideNearFooter();
  const examples = t('landing.search.examples', { returnObjects: true }) as string[];
  const { example } = useRotatingPlaceholder(examples, !reduce);

  // Fire the teaser-engagement event at most once per focus cycle: focus covers
  // keyboard + desktop click (a press is always preceded by focus), press covers
  // touch taps that don't move DOM focus. The flag resets on blur.
  const engaged = useRef(false);
  const captureEngaged = () => {
    if (engaged.current) return;
    engaged.current = true;
    posthog?.capture('landing_search_teaser_focused', { location: 'search_dock' });
  };

  return (
    <motion.div
      className={cn(
        'fixed inset-x-0 bottom-5 z-40 flex flex-col items-center gap-2.5 px-4',
        hidden && 'pointer-events-none',
      )}
      initial={reduce ? false : { opacity: 0, y: 16 }}
      animate={{ opacity: hidden ? 0 : 1, y: 0 }}
      transition={reduce ? { duration: 0 } : { duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
    >
      <TooltipTrigger delay={300}>
        <Button
          type="button"
          aria-disabled="true"
          aria-label={t('landing.search.label')}
          onPress={captureEngaged}
          onFocus={captureEngaged}
          onBlur={() => {
            engaged.current = false;
          }}
          className={cn(
            'group flex w-full max-w-md cursor-default items-center gap-3 rounded-full text-left',
            'border border-ink-200 bg-white/80 px-5 py-3.5 shadow-soft backdrop-blur grayscale',
            'outline-none data-[focus-visible]:ring-2 data-[focus-visible]:ring-ink-300',
            'data-[focus-visible]:ring-offset-2 data-[focus-visible]:ring-offset-cream-100',
          )}
        >
          <Icon name="sparkle" className="shrink-0 text-[18px] text-ink-400" />
          <span className="relative block flex-1 overflow-hidden">
            <AnimatePresence mode="wait" initial={false}>
              <motion.span
                key={example}
                className="block truncate text-[15px] text-ink-400"
                initial={reduce ? false : { opacity: 0, y: 8 }}
                animate={{ opacity: 1, y: 0 }}
                exit={reduce ? { opacity: 0 } : { opacity: 0, y: -8 }}
                transition={{ duration: 0.3 }}
              >
                {example}
              </motion.span>
            </AnimatePresence>
          </span>
        </Button>
        <Tooltip
          offset={10}
          className="rounded-lg bg-ink-900 px-3 py-1.5 text-xs font-semibold text-white shadow-lg"
        >
          {t('landing.search.hint')}
        </Tooltip>
      </TooltipTrigger>

      <FilterChips />
    </motion.div>
  );
}
