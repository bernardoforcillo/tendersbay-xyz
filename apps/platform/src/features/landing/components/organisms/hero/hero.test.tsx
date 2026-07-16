import { screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
  Link: ({ to, children }: { to: string; children?: ReactNode }) => <a href={to}>{children}</a>,
}));

import { Hero } from './index';

describe('Hero', () => {
  it('renders the headline, both CTAs and the trust line', () => {
    renderWithI18n(<Hero />, 'en-ie');
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      'The tender they already counted as theirs?',
    );
    // Primary CTA drives the signup route (the money action from the hero).
    expect(screen.getByRole('link', { name: /put your agents to work/i })).toHaveAttribute(
      'href',
      '/$locale/auth/signup',
    );
    // Secondary CTA stays the in-page scroll to the vision section.
    expect(screen.getByRole('link', { name: /see the vision/i })).toHaveAttribute(
      'href',
      '#vision',
    );
    expect(screen.getByText('27 countries, one search')).toBeInTheDocument();
  });
});
