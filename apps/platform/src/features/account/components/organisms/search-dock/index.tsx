import { MessageSquare } from 'lucide-react';
import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
import { useState } from 'react';
import { Button, Input, TextField, Tooltip, TooltipTrigger } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { Icon } from '~/features/landing/components/atoms';
import { useRotatingPlaceholder } from '~/features/landing/components/organisms/search-dock/use-rotating-placeholder';

export type SearchMode = 'search' | 'chat';

type SearchDockProps = {
  /** Which input mode the dock presents. Overview is always 'search'. */
  mode?: SearchMode;
  /** Controlled query value — pairs with `onChange` + `onSubmit` for the functional (Explore) dock. */
  value?: string;
  onChange?: (value: string) => void;
  /** Present + mode='search' switches the dock from decorative to a real search form. */
  onSubmit?: () => void;
  /** Present (and no `onSubmit`) makes the decorative pill pressable — e.g. Oggi routing to Explore. */
  onPress?: () => void;
};

// The pill's container classes, shared by every rendering so the layoutId morph
// keeps a stable visual shape across Oggi <-> Explore <-> functional-form states.
const PILL_BASE =
  'group flex w-full items-center gap-3 rounded-full border border-ink-200 bg-white/80 px-5 py-3.5 text-left shadow-soft backdrop-blur outline-none data-[focus-visible]:ring-2 data-[focus-visible]:ring-ink-300 data-[focus-visible]:ring-offset-2 data-[focus-visible]:ring-offset-cream-100';

/**
 * The account search dock. Rendered on both Overview (pinned bottom) and Explore
 * (centered). A shared `layoutId` lets motion animate the dock between the two
 * positions as the route changes — the LayoutGroup in `__root.tsx` keeps the
 * shared-layout context alive across the swap.
 *
 * Three renderings share the same outer `motion.div` (so the layoutId morph
 * always survives):
 * - `onSubmit` + `mode === 'search'`: a real search form (Explore, once wired up).
 * - `onPress` only: the decorative pill made pressable (Oggi, routes to Explore).
 * - neither: the original decorative, disabled pill (landing-seeded callers, chat mode).
 */
export function SearchDock({
  mode = 'search',
  value,
  onChange,
  onSubmit,
  onPress,
}: SearchDockProps) {
  const { t } = useTranslation();
  const reduce = useReducedMotion();
  const examples = t('landing.search.examples', { returnObjects: true }) as string[];
  const { example } = useRotatingPlaceholder(examples, !reduce);
  const chat = mode === 'chat';
  const [focused, setFocused] = useState(false);
  const functional = mode === 'search' && !!onSubmit;

  return (
    <motion.div
      layout={!reduce}
      layoutId={reduce ? undefined : 'account-search-dock'}
      transition={{ type: 'spring', bounce: 0.14, duration: 0.6 }}
      className="w-full max-w-xl"
    >
      {functional ? (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            onSubmit?.();
          }}
          // The form isn't focusable, so PILL_BASE's own data-focus-visible ring
          // never fires — light the whole pill when its text input is focused
          // (the submit button keeps its own brand ring).
          className={`${PILL_BASE} has-[input:focus-visible]:ring-2 has-[input:focus-visible]:ring-ink-300 has-[input:focus-visible]:ring-offset-2 has-[input:focus-visible]:ring-offset-cream-100`}
        >
          <Icon name="sparkle" className="shrink-0 text-[18px] text-ink-400" />
          <span className="relative block flex-1 overflow-hidden">
            <TextField
              value={value ?? ''}
              onChange={onChange ?? (() => {})}
              aria-label={t('landing.search.label')}
            >
              <Input
                placeholder={example}
                onFocus={() => setFocused(true)}
                onBlur={() => setFocused(false)}
                className="block w-full truncate bg-transparent text-[15px] text-ink-900 outline-none"
              />
            </TextField>
            {!focused && !value && (
              <AnimatePresence mode="wait" initial={false}>
                <motion.span
                  key={example}
                  aria-hidden="true"
                  className="pointer-events-none absolute inset-0 flex items-center truncate bg-white/80 text-[15px] text-ink-400"
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
          <Button
            type="submit"
            aria-label={t('shell.palette.trigger')}
            className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-brand-600 text-white outline-none transition-colors data-[hovered]:bg-brand-700 data-[pressed]:bg-brand-800 data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600 data-[focus-visible]:ring-offset-2 data-[focus-visible]:ring-offset-cream-100"
          >
            <Icon name="arrow-right" className="text-[16px]" />
          </Button>
        </form>
      ) : (
        <TooltipTrigger delay={300}>
          <Button
            type="button"
            aria-disabled={onPress ? undefined : 'true'}
            aria-label={t('landing.search.label')}
            onPress={onPress ?? (() => {})}
            className={`${PILL_BASE} ${onPress ? 'cursor-pointer' : 'cursor-default grayscale'}`}
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
      )}
    </motion.div>
  );
}
