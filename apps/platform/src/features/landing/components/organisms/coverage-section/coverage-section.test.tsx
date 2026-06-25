import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { CoverageSection } from './index';

describe('CoverageSection', () => {
  it('renders all 27 EU countries as flag buttons', () => {
    renderWithI18n(<CoverageSection />, 'en-ie');
    expect(screen.getAllByRole('button')).toHaveLength(27);
  });

  it('marks every country coming-soon in the teaser (none available)', () => {
    renderWithI18n(<CoverageSection />, 'en-ie');
    const buttons = screen.getAllByRole('button');
    expect(buttons.every((b) => /Coming soon/.test(b.getAttribute('aria-label') ?? ''))).toBe(true);
  });

  it('localizes country names via Intl.DisplayNames', () => {
    renderWithI18n(<CoverageSection />, 'it-it');
    expect(screen.getByRole('button', { name: /Italia/ })).toBeInTheDocument();
  });

  it('opens a card with the national portal when a flag is clicked', async () => {
    const user = userEvent.setup();
    renderWithI18n(<CoverageSection />, 'en-ie');
    await user.click(screen.getByRole('button', { name: /Italy/ }));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText('Acquisti in Rete (MEPA)')).toBeInTheDocument();
  });
});
