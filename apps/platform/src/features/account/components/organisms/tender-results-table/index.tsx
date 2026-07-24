import { useNavigate } from '@tanstack/react-router';
import { cn, Pill } from '@tendersbay/components/core';
import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  countryFlag,
  countryName,
  deadlineInfo,
  type FitTier,
  fitTierPillClassName,
  fitTierPillTone,
  formatTenderValue,
  tenderTitle,
} from '~/features/account/components/organisms/tender-feed';

type SortKey = 'fitTier' | 'value' | 'deadline';
type SortDir = 'asc' | 'desc';

const FIT_ORDER: Record<string, number> = { strong: 0, possible: 1, long_shot: 2 };

const FIT_LABEL: Record<FitTier, { key: string; defaultValue: string }> = {
  strong: { key: 'tenders.fit.tier.strong', defaultValue: 'Strong fit' },
  possible: { key: 'tenders.fit.tier.possible', defaultValue: 'Possible fit' },
  long_shot: { key: 'tenders.fit.tier.longShot', defaultValue: 'Long shot' },
};

export type TenderResultsTableProps = {
  tenders: TenderResult[];
};

function sortIcon(active: boolean, dir: SortDir) {
  if (!active) return <ArrowUpDown size={11} className="opacity-40" />;
  return dir === 'asc' ? <ArrowUp size={11} /> : <ArrowDown size={11} />;
}

export function TenderResultsTable({ tenders }: TenderResultsTableProps) {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const [sort, setSort] = useState<{ key: SortKey; dir: SortDir }>({ key: 'fitTier', dir: 'asc' });

  function toggleSort(key: SortKey) {
    setSort((prev) => ({
      key,
      dir: prev.key === key && prev.dir === 'asc' ? 'desc' : 'asc',
    }));
  }

  const sorted = [...tenders].sort((a, b) => {
    const mul = sort.dir === 'asc' ? 1 : -1;
    switch (sort.key) {
      case 'fitTier': {
        const ao = FIT_ORDER[a.fitTier] ?? 9;
        const bo = FIT_ORDER[b.fitTier] ?? 9;
        return (ao - bo) * mul;
      }
      case 'value': {
        if (a.value === 0n && b.value === 0n) return 0;
        if (a.value === 0n) return 1;
        if (b.value === 0n) return -1;
        return (a.value < b.value ? -1 : a.value > b.value ? 1 : 0) * mul;
      }
      case 'deadline': {
        if (!a.deadline && !b.deadline) return 0;
        if (!a.deadline) return 1;
        if (!b.deadline) return -1;
        return (a.deadline < b.deadline ? -1 : 1) * mul;
      }
      default:
        return 0;
    }
  });

  return (
    <div className="w-full overflow-x-auto rounded-2xl border border-cream-200 bg-white shadow-soft-sm">
      <table className="w-full min-w-[500px] border-collapse text-sm">
        <thead>
          <tr className="border-b border-cream-200">
            <th className="w-8 px-3 py-2.5" aria-label="Country" />
            <th className="px-3 py-2.5 text-left text-xs font-medium text-ink-500">
              {t('tenders.table.col.tender', { defaultValue: 'Tender' })}
            </th>
            <th className="px-3 py-2.5">
              <button
                type="button"
                onClick={() => toggleSort('fitTier')}
                className={cn(
                  'flex items-center gap-1 text-xs font-medium transition-colors',
                  sort.key === 'fitTier' ? 'text-ink-900' : 'text-ink-500 hover:text-ink-900',
                )}
              >
                {t('tenders.table.col.fit', { defaultValue: 'Fit' })}
                {sortIcon(sort.key === 'fitTier', sort.dir)}
              </button>
            </th>
            <th className="px-3 py-2.5">
              <button
                type="button"
                onClick={() => toggleSort('value')}
                className={cn(
                  'flex items-center gap-1 text-xs font-medium transition-colors',
                  sort.key === 'value' ? 'text-ink-900' : 'text-ink-500 hover:text-ink-900',
                )}
              >
                {t('tenders.table.col.value', { defaultValue: 'Value' })}
                {sortIcon(sort.key === 'value', sort.dir)}
              </button>
            </th>
            <th className="px-3 py-2.5">
              <button
                type="button"
                onClick={() => toggleSort('deadline')}
                className={cn(
                  'flex items-center gap-1 text-xs font-medium transition-colors',
                  sort.key === 'deadline' ? 'text-ink-900' : 'text-ink-500 hover:text-ink-900',
                )}
              >
                {t('tenders.table.col.deadline', { defaultValue: 'Deadline' })}
                {sortIcon(sort.key === 'deadline', sort.dir)}
              </button>
            </th>
          </tr>
        </thead>
        <tbody>
          {sorted.map((tender, i) => {
            const Flag = tender.country ? countryFlag(tender.country) : null;
            const origin = tender.country ? countryName(tender.country, i18n.language) : null;
            const { title, category } = tenderTitle(tender.title, tender.country);
            const dl = deadlineInfo(tender.deadline, new Date());
            const dlLabel = !dl
              ? null
              : dl.days < 0
                ? t('tenders.deadline.expired', { defaultValue: 'Closed' })
                : dl.days === 0
                  ? t('tenders.deadline.today', { defaultValue: 'Closes today' })
                  : t('tenders.deadline.days', {
                      count: dl.days,
                      defaultValue: '{{count}} days left',
                    });
            const value = formatTenderValue(tender.value, tender.currency, i18n.language);
            const fitTier = (tender.fitTier || undefined) as FitTier | undefined;

            return (
              <tr
                key={tender.id}
                tabIndex={0}
                onClick={() => void navigate({ to: '/tenders/$id', params: { id: tender.id } })}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ')
                    void navigate({ to: '/tenders/$id', params: { id: tender.id } });
                }}
                className={cn(
                  'group cursor-pointer transition-colors hover:bg-cream-50 focus-visible:bg-cream-50 focus-visible:outline-none',
                  i < sorted.length - 1 && 'border-b border-cream-100',
                )}
              >
                <td className="px-3 py-3">
                  {Flag ? (
                    <span
                      title={origin ?? undefined}
                      className="block w-5 overflow-hidden rounded ring-1 ring-ink-900/10"
                    >
                      <Flag aria-label={origin ?? undefined} className="block h-auto w-full" />
                    </span>
                  ) : (
                    <span className="font-mono text-[10px] uppercase text-ink-400">
                      {tender.country}
                    </span>
                  )}
                </td>
                <td className="max-w-[200px] px-3 py-3">
                  <p className="line-clamp-2 text-xs font-medium leading-snug text-ink-900 transition-colors group-hover:text-brand-700">
                    {title}
                  </p>
                  {category && (
                    <p className="mt-0.5 truncate text-[11px] text-ink-400">{category}</p>
                  )}
                  {tender.buyerName && (
                    <p className="mt-0.5 truncate text-[11px] text-ink-400">{tender.buyerName}</p>
                  )}
                </td>
                <td className="whitespace-nowrap px-3 py-3">
                  {fitTier ? (
                    <Pill tone={fitTierPillTone(fitTier)} className={fitTierPillClassName(fitTier)}>
                      {t(FIT_LABEL[fitTier].key, {
                        defaultValue: FIT_LABEL[fitTier].defaultValue,
                      })}
                    </Pill>
                  ) : (
                    <span className="text-[11px] text-ink-300">—</span>
                  )}
                </td>
                <td className="whitespace-nowrap px-3 py-3 text-right font-mono text-xs tabular-nums text-brand-700">
                  {value ?? <span className="font-sans text-[11px] text-ink-300">—</span>}
                </td>
                <td className="whitespace-nowrap px-3 py-3">
                  {dlLabel && dl ? (
                    <Pill tone={dl.tone}>{dlLabel}</Pill>
                  ) : (
                    <span className="text-[11px] text-ink-300">—</span>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
