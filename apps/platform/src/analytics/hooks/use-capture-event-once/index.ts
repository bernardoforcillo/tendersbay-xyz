import { useEffect, useRef } from 'react';
import type { AnalyticsEventName, AnalyticsEventProps } from '../../events';
import { useCaptureEvent } from '../use-capture-event';

/**
 * Fire a "shown"/"viewed" event once per distinct `key`.
 *
 * WHY this is a hook rather than a `useEffect` per component: three of the
 * scheda gara's events fire on mount (`document_coverage_shown`,
 * `eligibility_gap_shown`, `go_no_go_recommended`) and the surface they sit on
 * refetches deliberately — capturing an answer to an unknown gap re-runs the
 * eligibility check so the verdict can visibly flip in front of the user. Every
 * one of those refetches remounts nothing and changes no route, but it does
 * hand the component a new object. An unguarded mount effect would fire again
 * and inflate the denominator of every rate computed from these events, and the
 * override rate is the number this whole phase is measured by.
 *
 * `key` is the identity of the *snapshot* being reported on — an
 * `assessment.evaluatedAt`, a `tender.id`, a coverage tuple — not of the
 * component. A new snapshot is a new observation and fires again; the same
 * snapshot re-rendered never does. Pass `null`/`undefined` while the data is
 * still loading: nothing fires until there is something real to report.
 */
export function useCaptureEventOnce<K extends AnalyticsEventName>(
  name: K,
  key: string | null | undefined,
  props: AnalyticsEventProps[K],
): void {
  const capture = useCaptureEvent();

  // Read through a ref so the effect depends on the snapshot identity alone.
  // Depending on `props` would re-run the effect on every render (a fresh object
  // literal each time) and lean the whole guard on the ref comparison below,
  // which is exactly the fragility this hook exists to remove.
  const propsRef = useRef(props);
  propsRef.current = props;

  const firedKeyRef = useRef<string | null>(null);

  useEffect(() => {
    if (!key || firedKeyRef.current === key) {
      return;
    }
    firedKeyRef.current = key;
    capture(name, propsRef.current);
  }, [name, key, capture]);
}
