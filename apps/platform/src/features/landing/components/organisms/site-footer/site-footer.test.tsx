import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => vi.fn() }));

import { SiteFooter } from './index';

describe('SiteFooter', () => {
  it('renders the contact mailto link and copyright', () => {
    renderWithI18n(<SiteFooter />, 'en-ie');
    expect(screen.getByRole('contentinfo')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /get in touch/i })).toHaveAttribute(
      'href',
      'mailto:me@bernardoforcillo.com',
    );
    expect(screen.getByText('© Bernardo Forcillo — All rights reserved')).toBeInTheDocument();
  });
});
