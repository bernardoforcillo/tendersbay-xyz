import { MessageSquare } from 'lucide-react';
import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
import { Button, Tooltip, TooltipTrigger } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { Icon } from '~/features/landing/components/atoms';
import { useRotatingPlaceholder } from '~/features/landing/components/organisms/search-dock/use-rotating-placeholder';

export type SearchMode = 'search' | 'chat';

type SearchDockProps = {
  /** Which input mode the dock presents. Overview is always 'search'. */
  mode?: SearchMode;
};

/**
 * The account search dock. Rendered on both Overview (pinned bottom) and Explore
 * (centered). A shared `layoutId` lets motion animate the dock between the two
 * positions as the route changes — the LayoutGroup in `__root.tsx` keeps the
 * shared-layout context alive across the swap.
 */
export function SearchDock({ mode = 'search' }: SearchDockProps) {
  const { t } = useTranslation();
  const reduce = useReducedMotion();
  const examples = t('landing.search.examples', { returnObjects: true }) as string[];
  const { example } = useRotatingPlaceholder(examples, !reduce);
  const chat = mode === 'chat';

  return (
    <motion.div
      layout={!reduce}
      layoutId={reduce ? undefined : 'account-search-dock'}
      transition={{ type: 'spring', bounce: 0.14, duration: 0.6 }}
      className="w-full max-w-xl"
    >
      <TooltipTrigger delay={300}>
        <Button
          type="button"
          aria-disabled="true"
          aria-label={t('landing.search.label')}
          onPress={() => {}}
          className="group flex w-full cursor-default items-center gap-3 rounded-full border border-ink-200 bg-white/80 px-5 py-3.5 text-left shadow-soft backdrop-blur grayscale outline-none data-[focus-visible]:ring-2 data-[focus-visible]:ring-ink-300 data-[focus-visible]:ring-offset-2 data-[focus-visible]:ring-offset-cream-100"
        >
          {chat ? (
            <MessageSquare size={18} aria-hidden="true" className="shrink-0 text-ink-400" />
          ) : (
            <Icon name="sparkle" className="shrink-0 text-[18px] text-ink-400" />
          )}
          <span className="relative block flex-1 overflow-hidden">
            {chat ? (
              <span className="block truncate text-[15px] text-ink-400">
                {t('account.explore.chatPlaceholder', {
                  defaultValue: 'Ask anything about tenders…',
                })}
              </span>
            ) : (
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
            )}
          </span>
        </Button>
        <Tooltip
          offset={10}
          className="rounded-lg bg-ink-900 px-3 py-1.5 text-xs font-semibold text-white shadow-lg"
        >
          {t('landing.search.hint')}
        </Tooltip>
      </TooltipTrigger>
    </motion.div>
  );
}
