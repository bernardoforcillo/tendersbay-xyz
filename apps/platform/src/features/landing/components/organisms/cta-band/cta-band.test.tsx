import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => vi.fn() }));

import { CtaBand } from './index';

describe('CtaBand', () => {
  it('renders the CTA heading and a button linking to #top', () => {
    renderWithI18n(<CtaBand />, 'en-ie');
    expect(
      screen.getByRole('heading', { name: 'Be first when tendersbay goes live.' }),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Join the waitlist' })).toHaveAttribute('href', '#top');
  });
});
