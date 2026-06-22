import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({ useNavigate: () => vi.fn() }));

import { LandingPage } from './index';

describe('LandingPage', () => {
  it('renders the landing template and sets the document title', async () => {
    renderWithI18n(<LandingPage />, 'en-ie');
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Win your next tender');
    await waitFor(() => {
      expect(document.title).toBe('tendersbay — Win your next tender in Europe');
    });
  });
});
