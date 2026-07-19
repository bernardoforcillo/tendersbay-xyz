/** Mirrors the backend's tender.FitTier ("strong" | "possible" | "long_shot") — never a numeric percentage. */
export type FitTier = 'strong' | 'possible' | 'long_shot';

/** Mirrors the backend's tender.ReasonSignals — the FACTS behind a tier, not a prebuilt sentence. */
export type ReasonSignals = {
  sectorMatch: boolean;
  countryMatch: boolean;
  valueFit: string; // "in_band" | "below" | "above" | "unknown"
  deadlineDays: number; // meaningful only when hasDeadline is true
  hasDeadline: boolean;
  regionMatch: boolean;
  procedureMatch: boolean;
};

/** Strong is the only tier that earns the brand-colored "match" Pill tone; the rest stay neutral. */
export function fitTierPillTone(tier: FitTier): 'match' | 'neutral' {
  return tier === 'strong' ? 'match' : 'neutral';
}

/**
 * There are no neutral-gray tokens in this app's theme (see .claude/rules/frontend.md) — a
 * long-shot result recedes visually via the `grayscale` filter utility, the same technique
 * `country-flag`/`search-dock` already use for a "coming soon"/inactive look.
 */
export function fitTierPillClassName(tier: FitTier): string | undefined {
  return tier === 'long_shot' ? 'grayscale' : undefined;
}

export type ReasonFragment = { key: string; count?: number };

/**
 * Builds the ordered list of i18n keys behind a fit tier's reason line — the caller (the card
 * component) resolves each key via useTranslation and joins the results with " · ", matching
 * the card's existing value/status line. Returns an empty array when nothing matched (the
 * tier Pill alone is the only signal shown — never a fabricated reason).
 *
 * Order: sector, country, value band, deadline, then region and procedure type (appended last —
 * these two signals were added in a later amendment on top of the original four).
 */
export function fitReasonFragments(reason: ReasonSignals): ReasonFragment[] {
  const fragments: ReasonFragment[] = [];
  if (reason.sectorMatch) fragments.push({ key: 'tenders.fit.reasonSector' });
  if (reason.countryMatch) fragments.push({ key: 'tenders.fit.reasonCountry' });
  if (reason.valueFit === 'in_band') fragments.push({ key: 'tenders.fit.reasonValueInBand' });
  if (reason.valueFit === 'below') fragments.push({ key: 'tenders.fit.reasonValueBelow' });
  if (reason.valueFit === 'above') fragments.push({ key: 'tenders.fit.reasonValueAbove' });
  if (reason.hasDeadline)
    fragments.push({ key: 'tenders.deadline.days', count: reason.deadlineDays });
  if (reason.regionMatch) fragments.push({ key: 'tenders.fit.reasonRegion' });
  if (reason.procedureMatch) fragments.push({ key: 'tenders.fit.reasonProcedure' });
  return fragments;
}
