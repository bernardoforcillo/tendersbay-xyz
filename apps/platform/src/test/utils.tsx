import { type RenderResult, render } from '@testing-library/react';
import type { ReactElement } from 'react';
import { I18nextProvider } from 'react-i18next';
import { i18n } from '~/i18n';
import type { Locale } from '~/i18n/detect-locale';

export function renderWithI18n(ui: ReactElement, locale: Locale = 'en-ie'): RenderResult {
  void i18n.changeLanguage(locale);
  return render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>);
}
