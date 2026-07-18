import { cn } from '@tendersbay/components/core';
import { motion, useReducedMotion } from 'motion/react';
import { usePostHog } from 'posthog-js/react';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Icon } from '~/features/landing/components/atoms';
import { writeLandingCarryOver } from '~/lib/landing-carry-over';
import { SearchResults } from './search-results';
import { useHideNearFooter } from './use-hide-near-footer';
import { useLandingSearch } from './use-landing-search';
import { useRotatingPlaceholder } from './use-rotating-placeholder';

export function SearchDock() {
  const { t } = useTranslation();
  const posthog = usePostHog();
  const reduce = useReducedMotion();
  const hidden = useHideNearFooter();
  const examples = t('landing.search.examples', { returnObjects: true }) as string[];

  const [query, setQuery] = useState('');
  // Rotate the placeholder only while the field is empty — once you type, the
  // placeholder is gone anyway, so there's nothing to distract from.
  const { example } = useRotatingPlaceholder(examples, !reduce && query.length === 0);

  const { status, results } = useLandingSearch(query, {
    onResolved: ({ queryLength, resultCount }) => {
      posthog?.capture('landing_search_performed', {
        query_length: queryLength,
        result_count: resultCount,
        location: 'search_dock',
      });
      // Carry the resolved search over to the first-run client-profile capture
      // (shown once the visitor signs up and creates a workspace). The dock has
      // no filter controls today, so filters is always empty — the contract
      // still carries the field for a later filter UI to fill in.
      writeLandingCarryOver({ query: query.trim(), filters: {} });
    },
  });

  // Fire the search-engagement event at most once per focus cycle: focus covers
  // keyboard + desktop click. The flag resets on blur.
  const engaged = useRef(false);
  const captureFocused = () => {
    if (engaged.current) return;
    engaged.current = true;
    posthog?.capture('landing_search_focused', { location: 'search_dock' });
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
      {/* The dock is pinned to the viewport bottom, so results expand *upward*:
          they render above the input and stay on screen (Spotlight/Raycast pattern). */}
      <SearchResults status={status} results={results} />

      <div
        className={cn(
          'flex w-full max-w-md items-center gap-3 rounded-full',
          'border border-ink-200 bg-white/80 px-5 py-3.5 shadow-soft backdrop-blur',
          'focus-within:ring-2 focus-within:ring-ink-300 focus-within:ring-offset-2 focus-within:ring-offset-cream-100',
        )}
      >
        <Icon name="search" className="shrink-0 text-[18px] text-ink-400" />
        <input
          type="search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onFocus={captureFocused}
          onBlur={() => {
            engaged.current = false;
          }}
          aria-label={t('landing.search.label')}
          placeholder={example}
          className="min-w-0 flex-1 bg-transparent text-[15px] text-ink-900 outline-none placeholder:text-ink-400"
        />
      </div>
    </motion.div>
  );
}
