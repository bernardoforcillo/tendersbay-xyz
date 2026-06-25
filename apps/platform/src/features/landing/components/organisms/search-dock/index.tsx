import { motion, useReducedMotion } from 'motion/react';
import { Button, Tooltip, TooltipTrigger } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { Icon } from '~/features/landing/components/atoms';
import { cx } from '~/features/landing/cx';
import { useHideNearFooter } from './use-hide-near-footer';

export function SearchDock() {
  const { t } = useTranslation();
  const reduce = useReducedMotion();
  const hidden = useHideNearFooter();

  return (
    <motion.div
      className={cx(
        'fixed inset-x-0 bottom-5 z-40 flex justify-center px-4',
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
          aria-label={t('landing.search.ariaLabel')}
          onPress={() => {}}
          className={cx(
            'group flex w-full max-w-md cursor-default items-center gap-3 rounded-full text-left',
            'border border-cream-300/80 bg-white/90 px-5 py-3.5 shadow-soft-lg backdrop-blur',
            'outline-none data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600',
            'data-[focus-visible]:ring-offset-2 data-[focus-visible]:ring-offset-cream-100',
          )}
        >
          <Icon name="search" className="shrink-0 text-[18px] text-brand-600" />
          <span className="flex-1 truncate text-[15px] text-ink-500">
            {t('landing.search.placeholder')}
          </span>
          <span className="shrink-0 rounded-full bg-brand-50 px-2.5 py-1 font-mono text-[11px] font-semibold tracking-tight text-brand-700">
            {t('landing.search.badge')}
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
