import { screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
  Link: ({ to, children }: { to: string; children?: ReactNode }) => <a href={to}>{children}</a>,
}));

import { LanguageSwitcher } from './index';

describe('LanguageSwitcher', () => {
  it('exposes an accessible language selector showing the current locale', () => {
    renderWithI18n(<LanguageSwitcher />, 'en-ie');
    const trigger = screen.getByRole('button', { name: /language/i });
    expect(trigger).toHaveTextContent('English');
  });
});
