import { cn } from '@tendersbay/components/core';
import { useState } from 'react';
import { Button, Tooltip, TooltipTrigger } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import type { AttributionView } from '~/features/company/constants';

export type ProvenanceMarkProps = {
  attribution?: AttributionView;
  /** Optional hover/focus explanation, e.g. when the fact was asserted. */
  detail?: string;
  className?: string;
};

/**
 * How a dossier fact came to be believed: you typed it, you confirmed it, the
 * agent inferred it, or it was imported.
 *
 * This is the entire reason `Attribution` exists on the wire. A dossier that
 * cannot say which of its facts a human vouched for is a dossier whose verdicts
 * nobody can audit — and an agent-inferred SOA classifica presented with the same
 * confidence as one the user typed is precisely the kind of quiet overclaim this
 * product is built to avoid.
 *
 * A missing attribution renders "never asserted", NOT "unknown provenance". The
 * two are different states: one means nobody has ever said anything about this
 * field, the other means someone did and we lost track of who.
 */
export function ProvenanceMark({ attribution, detail, className }: ProvenanceMarkProps) {
  const { t } = useTranslation();
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);

  const provenance = attribution?.provenance ?? '';
  const label = provenance
    ? t(`company.provenance.${provenance}`, PROVENANCE_FALLBACK[provenance] ?? provenance)
    : t('company.provenance.never', 'Never asserted');

  const base = cn(
    'inline-flex items-center rounded-full bg-cream-100 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-ink-500',
    !provenance && 'grayscale',
    className,
  );

  if (!detail) return <span className={base}>{label}</span>;

  return (
    // Controlled open state, not `delay={0}`: RAC's TooltipTrigger has a ~1.5s
    // GLOBAL hover warmup that delay does not bypass, so an uncontrolled tooltip
    // reads as broken on the first hover of a page.
    <TooltipTrigger isOpen={hovered || focused}>
      <Button
        onHoverChange={setHovered}
        onFocusChange={setFocused}
        // Not `isDisabled`: that would drop the mark from the tab order and take
        // its explanation with it. It carries no action, so pressing is a no-op.
        aria-disabled="true"
        onPress={() => {}}
        className={cn(
          base,
          'cursor-default outline-none data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600',
        )}
      >
        {label}
      </Button>
      <Tooltip className="rounded-lg bg-ink-900 px-2 py-1 text-xs text-cream-50 shadow-soft">
        {detail}
      </Tooltip>
    </TooltipTrigger>
  );
}

const PROVENANCE_FALLBACK: Record<string, string> = {
  user_stated: 'You entered this',
  user_confirmed: 'You confirmed this',
  agent_inferred: 'The agent inferred this',
  imported: 'Imported',
};
