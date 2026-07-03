import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { LoginPage } from './index';

vi.mock('~/lib/api/client', () => ({ authClient: { login: vi.fn() } }));
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
  useParams: () => ({ locale: 'en-ie' }),
  Link: ({ to, children }: { to: string; children?: ReactNode }) => <a href={to}>{children}</a>,
}));
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_k: string, fallback: string) => fallback,
    i18n: { language: 'en-ie' },
  }),
}));

describe('LoginPage', () => {
  it('renders email and password fields', () => {
    render(<LoginPage />);
    expect(screen.getByRole('textbox', { name: /email/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
  });
});
