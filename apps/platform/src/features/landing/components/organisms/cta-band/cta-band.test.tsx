import { screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
  Link: ({ to, children }: { to: string; children?: ReactNode }) => <a href={to}>{children}</a>,
}));

import { CtaBand } from './index';

describe('CtaBand', () => {
  it('renders the CTA heading and a button linking to the signup route', () => {
    renderWithI18n(<CtaBand />, 'en-ie');
    expect(
      screen.getByRole('heading', {
        name: 'Your agents are ready. The only thing missing is you.',
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Claim your spot' })).toHaveAttribute(
      'href',
      '/$locale/auth/signup',
    );
  });
});
