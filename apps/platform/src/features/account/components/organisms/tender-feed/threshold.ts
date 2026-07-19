/**
 * Maps the backend's coarse `eu_threshold` band onto the card badge — the i18n label key and a
 * visual tone. Mirrors `tender.euThreshold` ("below_eu" | "above_eu" | ""): `below_eu` is the
 * SME-winnable / emphasised side, `above_eu` recedes as muted. Every other value — the empty
 * string when the value is unknown or the band is buyer-type ambiguous, or any unexpected string
 * — returns `null`, so the card renders NOTHING rather than an unfounded claim (the guardrail).
 * This is a distinct axis from the fit tier's `value_fit` — never reuse its below/above/in_band
 * vocabulary here.
 */
export function thresholdBadge(band: string): { labelKey: string; tone: 'below' | 'above' } | null {
  switch (band) {
    case 'below_eu':
      return { labelKey: 'tenders.threshold.belowEu', tone: 'below' };
    case 'above_eu':
      return { labelKey: 'tenders.threshold.aboveEu', tone: 'above' };
    default:
      return null; // "" / unknown → no badge (the guardrail)
  }
}
