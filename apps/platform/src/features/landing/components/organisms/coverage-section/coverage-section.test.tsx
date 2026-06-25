import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { CoverageSection } from './index';

describe('CoverageSection', () => {
  it('renders all 27 EU countries as list items', () => {
    renderWithI18n(<CoverageSection />, 'en-ie');
    expect(screen.getAllByRole('listitem')).toHaveLength(27);
  });

  it('marks every country coming-soon in the teaser (none available)', () => {
    renderWithI18n(<CoverageSection />, 'en-ie');
    expect(screen.getAllByText(/Coming soon/)).toHaveLength(27);
  });

  it('localizes country names via Intl.DisplayNames', () => {
    renderWithI18n(<CoverageSection />, 'it-it');
    expect(screen.getByText(/Italia/)).toBeInTheDocument();
  });
});
