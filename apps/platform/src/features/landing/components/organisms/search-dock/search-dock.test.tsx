import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { SearchDock } from './index';

describe('SearchDock', () => {
  it('renders the localized placeholder and the coming-soon badge', () => {
    renderWithI18n(<SearchDock />, 'en-ie');
    expect(screen.getByText("Soon you'll be able to search…")).toBeInTheDocument();
    expect(screen.getByText('Coming soon')).toBeInTheDocument();
  });

  it('exposes a focusable, non-actionable control labelled for assistive tech', () => {
    renderWithI18n(<SearchDock />, 'en-ie');
    const control = screen.getByRole('button', { name: 'Search — coming soon' });
    expect(control).toHaveAttribute('aria-disabled', 'true');
    control.focus();
    expect(control).toHaveFocus();
  });

  it('localizes the placeholder (it-it)', () => {
    renderWithI18n(<SearchDock />, 'it-it');
    expect(screen.getByText('Presto potrai cercare…')).toBeInTheDocument();
  });
});
