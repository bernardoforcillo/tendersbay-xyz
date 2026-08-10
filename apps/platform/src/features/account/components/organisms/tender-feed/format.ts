/**
 * Formats a tender's contract value for the given locale.
 *
 * `value` is in MINOR units (cents) — the unit `tenders.value` is stored in and
 * the unit the wire carries, set at ingestion by parseMinorUnits. It is divided
 * by 100 here, at the one place a figure becomes text, so a €6,297,240.05
 * notice reads as €6,297,240.05 and not as €629,724,005.
 *
 * Cents are shown only when the notice actually carries them: procurement
 * values are usually whole euros, where trailing ",00" is noise, but an eForms
 * amount like "22134549.01" is exact and rounding it away would misstate a
 * figure bidders compare against a threshold.
 *
 * Returns null for a non-positive value or a missing currency (many tenders
 * have no published estimate), and also null — instead of throwing — when
 * `currency` isn't a valid ISO 4217 code, since that's out of our control
 * (source data from the connectors).
 */
export function formatTenderValue(value: bigint, currency: string, locale: string): string | null {
  if (value <= 0n || !currency) return null;

  try {
    return new Intl.NumberFormat(locale, {
      style: 'currency',
      currency,
      minimumFractionDigits: 0,
      maximumFractionDigits: 2,
      // Number() before the divide, not `value / 100n`, because bigint division
      // truncates the cents this exists to preserve. Contract values are far
      // below 2^53 minor units, so the conversion is exact, and Intl rounds the
      // quotient to 2 decimals — well inside the representation error.
    }).format(Number(value) / 100);
  } catch {
    return null;
  }
}
