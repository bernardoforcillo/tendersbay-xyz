import { Card } from '@tendersbay/components/core';
import type { ReactNode } from 'react';

export type SettingsSectionProps = {
  title: string;
  description?: string;
  children: ReactNode;
  variant?: 'default' | 'danger';
};

export function SettingsSection({
  title,
  description,
  children,
  variant = 'default',
}: SettingsSectionProps) {
  const isDanger = variant === 'danger';
  return (
    <div className="grid grid-cols-1 gap-x-8 gap-y-6 py-8 md:grid-cols-3">
      <div>
        <h2 className={`text-base font-semibold ${isDanger ? 'text-red-700' : 'text-ink-900'}`}>
          {title}
        </h2>
        {description && <p className="mt-1 text-sm leading-relaxed text-ink-500">{description}</p>}
      </div>
      <div className="md:col-span-2">
        <Card
          padding="md"
          className={isDanger ? 'border border-red-200 bg-red-50/40 shadow-none' : undefined}
        >
          {children}
        </Card>
      </div>
    </div>
  );
}
