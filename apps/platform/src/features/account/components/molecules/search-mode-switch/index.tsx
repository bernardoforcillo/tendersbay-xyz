import { MessageSquare, Search } from 'lucide-react';
import { motion } from 'motion/react';
import type { ComponentType } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import type { SearchMode } from '~/features/account/components/organisms';

type SearchModeSwitchProps = {
  mode: SearchMode;
  onChange: (mode: SearchMode) => void;
};

const MODES: {
  id: SearchMode;
  icon: ComponentType<{ size?: number; className?: string; 'aria-hidden'?: boolean }>;
  labelKey: string;
  fallback: string;
}[] = [
  { id: 'search', icon: Search, labelKey: 'account.explore.modeSearch', fallback: 'Search' },
  { id: 'chat', icon: MessageSquare, labelKey: 'account.explore.modeChat', fallback: 'Chat' },
];

export function SearchModeSwitch({ mode, onChange }: SearchModeSwitchProps) {
  const { t } = useTranslation();

  return (
    <div className="inline-flex items-center gap-1 rounded-full bg-cream-200 p-1">
      {MODES.map(({ id, icon: Icon, labelKey, fallback }) => {
        const active = id === mode;
        return (
          <Button
            key={id}
            onPress={() => onChange(id)}
            aria-pressed={active}
            className="relative flex items-center gap-2 rounded-full px-4 py-1.5 text-sm font-medium outline-none transition-colors data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600"
          >
            {active && (
              <motion.span
                layoutId="search-mode-indicator"
                aria-hidden="true"
                className="absolute inset-0 rounded-full bg-white shadow-soft"
                transition={{ type: 'spring', stiffness: 300, damping: 30 }}
              />
            )}
            <Icon
              size={15}
              aria-hidden={true}
              className={`relative z-10 ${active ? 'text-ink-900' : 'text-ink-400'}`}
            />
            <span className={`relative z-10 ${active ? 'text-ink-900' : 'text-ink-500'}`}>
              {t(labelKey, { defaultValue: fallback })}
            </span>
          </Button>
        );
      })}
    </div>
  );
}
