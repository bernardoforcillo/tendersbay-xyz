/** Convert our lowercase locale code (e.g. `en-ie`) to BCP-47 hreflang casing (`en-IE`). */
export function bcp47(locale: string): string {
  const [language, region] = locale.split('-');
  if (!language) {
    return locale;
  }
  return region ? `${language}-${region.toUpperCase()}` : language;
}
