import { motion, useReducedMotion } from 'motion/react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SearchModeSwitch } from '~/features/account/components/molecules';
import { SearchDock, type SearchMode } from '~/features/account/components/organisms';
import { AccountLayout } from '~/features/account/components/templates/account-layout';
import { useAuthStore } from '~/store/auth';

export function AccountExplorePage() {
  const { t } = useTranslation();
  const reduce = useReducedMotion();
  const user = useAuthStore((s) => s.user);
  const name = user?.displayName?.split(' ')[0];
  const [mode, setMode] = useState<SearchMode>('search');

  return (
    <AccountLayout>
      {/* Centered empty-state — ChatGPT / Gemini / Claude layout. The dock animates
          up from Overview's bottom position via its shared layoutId. */}
      <div className="flex min-h-full flex-1 flex-col items-center justify-center gap-6 px-4 pb-16">
        <motion.div
          initial={reduce ? false : { opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4, delay: 0.18, ease: [0.22, 1, 0.36, 1] }}
        >
          <SearchModeSwitch mode={mode} onChange={setMode} />
        </motion.div>
        <motion.h1
          initial={reduce ? false : { opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.45, delay: 0.12, ease: [0.22, 1, 0.36, 1] }}
          className="text-center text-2xl font-semibold text-ink-900 sm:text-3xl"
        >
          {name
            ? t('account.explore.greetingNamed', {
                defaultValue: 'What are you bidding on today, {{name}}?',
                name,
              })
            : t('account.explore.greeting', { defaultValue: 'What are you bidding on today?' })}
        </motion.h1>
        <SearchDock mode={mode} />
      </div>
    </AccountLayout>
  );
}
