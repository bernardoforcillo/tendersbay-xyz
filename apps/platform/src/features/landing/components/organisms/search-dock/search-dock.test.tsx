import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { SearchDock } from './index';

describe('SearchDock', () => {
  it('renders the first localized example as the looping placeholder', () => {
    renderWithI18n(<SearchDock />, 'en-ie');
    expect(screen.getByText('Public school renovations')).toBeInTheDocument();
  });

  it('exposes a focusable, disabled, grayscale control labelled for assistive tech', () => {
    renderWithI18n(<SearchDock />, 'en-ie');
    const control = screen.getByRole('button', { name: 'Search' });
    expect(control).toHaveAttribute('aria-disabled', 'true');
    expect(control.className).toContain('grayscale');
    control.focus();
    expect(control).toHaveFocus();
  });

  it('localizes the example placeholder (it-it)', () => {
    renderWithI18n(<SearchDock />, 'it-it');
    expect(screen.getByText('Ristrutturazione di scuole pubbliche')).toBeInTheDocument();
  });

  it('no longer shows the old teaser placeholder', () => {
    renderWithI18n(<SearchDock />, 'en-ie');
    expect(screen.queryByText("Soon you'll be able to search…")).not.toBeInTheDocument();
  });

  it('renders the four localized, disabled-but-focusable filter chips', () => {
    renderWithI18n(<SearchDock />, 'en-ie');
    for (const label of ['Country', 'Sector', 'Deadline', 'Value']) {
      const chip = screen.getByRole('button', { name: label });
      expect(chip).toHaveAttribute('aria-disabled', 'true');
      expect(chip.className).toContain('grayscale');
      chip.focus();
      expect(chip).toHaveFocus();
    }
  });

  it('localizes the filter chips (it-it)', () => {
    renderWithI18n(<SearchDock />, 'it-it');
    expect(screen.getByRole('button', { name: 'Paese' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Scadenza' })).toBeInTheDocument();
  });
});
