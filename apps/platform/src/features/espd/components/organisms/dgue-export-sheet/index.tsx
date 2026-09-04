import { Code, ConnectError } from '@connectrpc/connect';
import { Banner, Card, cn } from '@tendersbay/components/core';
import type { Export } from '@tendersbay/proto/espd/v1/espd_pb';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { AnalyticsLocation } from '~/analytics';
import { useCaptureEvent } from '~/analytics';
import type { TranslateFn } from '~/features/espd/components/molecules/dgue-gap-row';
import type { EspdFormat, EspdVersion } from '~/features/espd/constants';
import { espdClient } from '~/lib/api/client';

export type DgueExportSheetProps = {
  workbenchId: string;
  bidId: string;
  /** Only offered on a document with nothing missing. */
  ready: boolean;
  requestKnown: boolean;
  canManage: boolean;
  /** RFC3339; drives the peak-end line. Empty when the tender has no deadline. */
  deadline?: string;
  exports: Export[];
  /** Analytics surface. */
  location: AnalyticsLocation;
  /** Refresh the export history after a successful export. */
  onExported: () => void;
  className?: string;
};

type Choice = { version: EspdVersion; format: EspdFormat };

const CHOICES: Choice[] = [
  { version: 'edm_2_1_1', format: 'pdf' },
  { version: 'edm_2_1_1', format: 'xml' },
  { version: 'edm_4', format: 'xml' },
];

/**
 * The three files, and the sentence that says what to do with them.
 *
 * Order is the content. The PDF is FIRST and is the only one described as the
 * document: it carries every value, it is what a legal representative signs, and
 * it is the one artefact that works at every contracting authority in Europe.
 * The two XMLs follow, described as what they are — for re-import where a portal
 * supports it — because presenting an XML as "the thing you submit" would be a
 * claim this product has not verified for any specific portal.
 *
 * The strip underneath is the peak-end note: the last thing a person reads here
 * is what happens NEXT, which is a signature they must obtain and a deadline
 * they must beat. Ending on a download button would leave the impression that
 * downloading was the finish line.
 */
export function DgueExportSheet({
  workbenchId,
  bidId,
  ready,
  requestKnown,
  canManage,
  deadline,
  exports,
  location,
  onExported,
  className,
}: DgueExportSheetProps) {
  const { t } = useTranslation();
  const capture = useCaptureEvent();
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function run(choice: Choice) {
    // Underscore, not a colon: i18next reads `:` as its namespace separator, so
    // a key containing one silently resolves against a namespace that does not
    // exist and renders the fallback in every language.
    const key = `${choice.version}_${choice.format}`;
    setError(null);
    setBusy(key);
    try {
      const out = await espdClient.exportResponse({
        workbenchId,
        bidId,
        version: choice.version,
        format: choice.format,
        // The document's own language follows the reader's UI, which is the
        // person who will hand it to a commission.
        locale: document.documentElement.lang || '',
      });
      save(out.content, out.filename, out.mimeType);
      capture('dgue_exported', {
        location,
        version: choice.version,
        format: choice.format,
        request_known: requestKnown,
      });
      onExported();
    } catch (e: unknown) {
      setError(messageFor(e, t));
    } finally {
      setBusy(null);
    }
  }

  return (
    <Card className={className}>
      <h2 className="font-semibold text-ink-700 text-sm">{t('espd.export.title', 'Export')}</h2>

      {!ready && (
        <p className="mt-2 text-ink-500 text-sm">
          {t(
            'espd.export.notReadyHint',
            'The export unlocks once nothing is missing and your declarations are confirmed for this tender.',
          )}
        </p>
      )}

      {error && (
        <Banner tone="error" className="mt-3">
          {error}
        </Banner>
      )}

      {canManage && (
        <div className="mt-3 flex flex-col gap-2">
          {CHOICES.map((choice) => {
            const key = `${choice.version}_${choice.format}`;
            const disabled = !ready || busy !== null;
            return (
              <button
                key={key}
                type="button"
                onClick={() => void run(choice)}
                aria-disabled={disabled ? 'true' : undefined}
                className={cn(
                  choice.format === 'pdf' ? PRIMARY_BUTTON : SECONDARY_BUTTON,
                  'justify-between gap-3 text-left',
                  disabled && 'cursor-default opacity-50',
                )}
              >
                <span>
                  {busy === key
                    ? t('espd.export.exporting', 'Preparing…')
                    : t(`espd.export.choice.${key}`, CHOICE_FALLBACK[key] ?? key)}
                </span>
                <span
                  className={cn(
                    'text-xs',
                    choice.format === 'pdf' ? 'text-white/80' : 'text-ink-400',
                  )}
                >
                  {t(`espd.export.hint.${choice.format}`, HINT_FALLBACK[choice.format] ?? '')}
                </span>
              </button>
            );
          })}
        </div>
      )}

      {/* Peak-end: what happens after the download, said before they leave. */}
      <p className="mt-4 rounded-xl bg-cream-100 p-3 text-ink-600 text-sm">
        {deadline
          ? t('espd.export.nextStep', {
              when: formatDate(deadline),
              defaultValue:
                'Next: your legal representative signs it electronically, then you upload it to the contracting authority’s portal before {{when}}.',
            })
          : t(
              'espd.export.nextStepNoDeadline',
              'Next: your legal representative signs it electronically, then you upload it to the contracting authority’s portal.',
            )}
      </p>

      {exports.length > 0 && (
        <details className="mt-4">
          <summary className="cursor-pointer font-semibold text-ink-700 text-xs uppercase tracking-wide">
            {t('espd.export.history', 'Previous exports')}
          </summary>
          <ul className="mt-2 flex list-none flex-col p-0">
            {exports.map((e) => (
              <li
                key={e.id}
                className="flex items-baseline justify-between gap-3 border-cream-200 border-b py-1.5 last:border-b-0"
              >
                <span className="text-ink-700 text-sm">
                  {t(`espd.export.choice.${e.version}_${e.format}`, `${e.version} ${e.format}`)}
                </span>
                <span className="shrink-0 text-ink-500 text-xs">{formatDate(e.exportedAt)}</span>
              </li>
            ))}
          </ul>
          <p className="mt-2 text-ink-400 text-xs">
            {t(
              'espd.export.historyHint',
              'We record that you exported, never the file itself. Exporting again rebuilds it from today’s dossier.',
            )}
          </p>
        </details>
      )}
    </Card>
  );
}

/**
 * Hands the bytes to the browser.
 *
 * The object URL is revoked on the next frame rather than immediately: Safari
 * reads the href asynchronously after the click, and revoking synchronously
 * gives that reader an empty blob and a silently corrupt download.
 */
function save(content: Uint8Array, filename: string, mimeType: string): void {
  const blob = new Blob([content as BlobPart], { type: mimeType || 'application/octet-stream' });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename || 'dgue';
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  requestAnimationFrame(() => URL.revokeObjectURL(url));
}

/**
 * The three refusals a person can actually act on, told apart.
 *
 * `permission_denied` here is not "you may not touch this tender" — the caller
 * already passed that gate — it is "your plan does not include the export", and
 * saying it as an access error would send someone to their admin instead of to
 * the pricing page.
 */
function messageFor(e: unknown, t: TranslateFn): string {
  if (e instanceof ConnectError) {
    if (e.code === Code.PermissionDenied) {
      return t(
        'espd.export.notEntitled',
        'The DGUE export is part of the Pro plan. Everything you see above stays free.',
      );
    }
    if (e.code === Code.FailedPrecondition) {
      return t(
        'espd.export.notReady',
        'Something is still missing, or your declarations need re-confirming for this tender.',
      );
    }
    return e.rawMessage;
  }
  return e instanceof Error ? e.message : t('espd.export.error', 'Could not export — try again.');
}

function formatDate(rfc3339: string): string {
  const parsed = new Date(rfc3339);
  if (Number.isNaN(parsed.getTime())) return rfc3339;
  return parsed.toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' });
}

/**
 * Four labels for three buttons: the history renders whatever the server
 * recorded, and the server will render a PDF against either data model. The
 * sheet does not offer that combination — one PDF is enough, and it is the
 * 2.1.1 one — but a row that predates a change here must still read as a
 * document rather than as `edm_4_pdf`.
 */
const CHOICE_FALLBACK: Record<string, string> = {
  edm_2_1_1_pdf: 'PDF — the document to sign',
  edm_2_1_1_xml: 'XML — ESPD 2.1.1 (eDGUE-IT)',
  edm_4_xml: 'XML — ESPD 4.x (eForms)',
  edm_4_pdf: 'PDF — ESPD 4.x',
};

const HINT_FALLBACK: Record<EspdFormat, string> = {
  pdf: 'Complete, in your language',
  xml: 'For reuse and import where supported',
};

const PRIMARY_BUTTON =
  'inline-flex h-11 items-center rounded-xl bg-brand-600 px-4 font-semibold text-sm text-white ' +
  'outline-none transition-colors duration-150 hover:bg-brand-700 focus-visible:ring-2 focus-visible:ring-brand-600 focus-visible:ring-offset-2';

const SECONDARY_BUTTON =
  'inline-flex h-11 items-center rounded-xl border border-cream-300 px-4 font-semibold text-ink-700 text-sm ' +
  'outline-none transition-colors duration-150 hover:bg-cream-200 focus-visible:ring-2 focus-visible:ring-brand-600';
