import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

export type SchedaGaraProps = {
  /** Title, buyer, fit and deadline — or the tombstone when the listing is gone. */
  header: ReactNode;
  /** What we have actually read. Omitted only when nothing is known about coverage. */
  coverage?: ReactNode;
  /** Recommendation, go/no-go, and the recorded override. */
  decision: ReactNode;
  /** Requirements checked against the dossier, with just-in-time capture. */
  gaps?: ReactNode;
  /** Where the points are won. */
  criteria?: ReactNode;
  /** ESPD/DGUE. */
  checklist: ReactNode;
  stage: ReactNode;
  /** Only for a `go` bid. */
  outcome?: ReactNode;
  /** The assistant, collapsed. */
  repairChat: ReactNode;
};

/**
 * The scheda gara: one gara, everything needed to decide on it, in one order.
 *
 * The ORDER is the content here, and it is encoded exactly once — in this file —
 * for the same reason the server encodes its recommendation's order once: so no
 * consumer re-decides it independently and the product does not end up arguing
 * with itself about what a reader should see first.
 *
 * Why this order:
 *
 *  1. **Header** — which gara, and when it closes.
 *  2. **Coverage before the verdict.** A verdict's credibility is a function of
 *     what was read. Putting "notice only" after "no-go" lets the verdict land
 *     unqualified and then footnote itself.
 *  3. **Decision** — the recommendation, the fact it rests on, the button.
 *  4. **Gaps** — the requirements, their evidence, and the questions that would
 *     move the verdict. They follow the decision because they explain it.
 *  5. **Criteria** — where the points are, for a bid that is going ahead.
 *  6. **Checklist**, 7. **stage/outcome** — the mechanics, once the decision is made.
 *  8. **Repair chat**, collapsed and last.
 *
 * Each block is its own labelled `<section>` so a screen-reader user can jump
 * between them, mirroring the public tender detail template.
 */
export function SchedaGara({
  header,
  coverage,
  decision,
  gaps,
  criteria,
  checklist,
  stage,
  outcome,
  repairChat,
}: SchedaGaraProps) {
  const { t } = useTranslation();

  return (
    <article className="mx-auto flex w-full max-w-2xl flex-col gap-6">
      {header}

      {/* mb-8 — the coverage strip and the verdict are independent claims, so the
          page's largest gap sits between them, owned by the strip. */}
      {coverage && (
        <section aria-label={t('bid.scheda.coverageTitle', 'What we have read')} className="mb-2">
          {coverage}
        </section>
      )}

      <section aria-label={t('bid.detail.goNoGoTitle', 'Decision')}>{decision}</section>

      {gaps && (
        <section aria-label={t('bid.scheda.evidenceTitle', 'Requirements and evidence')}>
          {gaps}
        </section>
      )}

      {criteria && (
        <section aria-label={t('bid.scheda.pointsTitle', 'How the points are awarded')}>
          {criteria}
        </section>
      )}

      <section aria-label={t('bid.checklist.title', 'ESPD / DGUE checklist')}>{checklist}</section>

      <section aria-label={t('bid.detail.stageTitle', 'Stage')}>{stage}</section>

      {outcome && <section aria-label={t('bid.detail.outcomeTitle', 'Outcome')}>{outcome}</section>}

      <section aria-label={t('bid.scheda.repairChat.title', 'Ask about this gara')}>
        {repairChat}
      </section>
    </article>
  );
}
