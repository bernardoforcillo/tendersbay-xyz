import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { ProofStrip } from './index';

describe('ProofStrip', () => {
  it('renders the lead, the three sourced figures and the citation', () => {
    const { container } = renderWithI18n(<ProofStrip />, 'en-ie');
    expect(container.querySelector('#proof')).not.toBeNull();

    // The three sourced values — the only numbers allowed on the page.
    expect(screen.getByText('€2 trillion+')).toBeInTheDocument();
    expect(screen.getByText('250,000+')).toBeInTheDocument();
    expect(screen.getByText('~800,000')).toBeInTheDocument();

    // Visible citation is the trust signal (we cite, we don't invent).
    expect(
      screen.getByText('European Commission · TED (Tenders Electronic Daily)'),
    ).toBeInTheDocument();

    // Exactly three stat items.
    expect(container.querySelectorAll('ul > li')).toHaveLength(3);
  });
});
