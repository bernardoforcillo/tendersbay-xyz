/**
 * Parses a free-text, comma-separated NUTS-prefix field into a clean list —
 * trim, uppercase, drop empties. There is no fixed, enumerable list of NUTS
 * codes in this frontend (a large hierarchical taxonomy, unlike the small
 * closed sets sectors/countries use), so the region control is a plain text
 * input rather than a toggle-chip list. This is a light format nudge only —
 * the backend's `regionRe` (`^[A-Z]{2}[A-Z0-9]{0,3}$`) is the source of
 * truth, and its `ErrInvalidRegion` surfaces through the form's existing
 * error Banner rather than being duplicated here.
 */
export function parseRegions(input: string): string[] {
  return input
    .split(',')
    .map((region) => region.trim().toUpperCase())
    .filter((region) => region.length > 0);
}

/** Inverse of `parseRegions`, for pre-filling the free-text field from a saved profile. */
export function formatRegions(regions: string[]): string {
  return regions.join(', ');
}
