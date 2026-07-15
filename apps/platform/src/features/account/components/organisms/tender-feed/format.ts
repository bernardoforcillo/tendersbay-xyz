/**
 * Formats a tender's contract value as whole-unit currency for the given
 * locale. Returns null for a non-positive value or a missing currency (many
 * tenders have no published estimate), and also null — instead of throwing —
 * when `currency` isn't a valid ISO 4217 code, since that's out of our
 * control (source data from the connectors).
 */
export function formatTenderValue(value: bigint, currency: string, locale: string): string | null {
  if (value <= 0n || !currency) return null;

  try {
    return new Intl.NumberFormat(locale, {
      style: 'currency',
      currency,
      maximumFractionDigits: 0,
    }).format(Number(value));
  } catch {
    return null;
  }
}
