import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => vi.fn() }));

import { CtaBand } from './index';

describe('CtaBand', () => {
  it('renders the CTA heading and a button linking to #top', () => {
    renderWithI18n(<CtaBand />, 'en-ie');
    expect(
      screen.getByRole('heading', {
        name: 'Your agents are ready. The only thing missing is you.',
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Claim your spot' })).toHaveAttribute('href', '#top');
  });
});
