import { screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
  Link: ({ to, children }: { to: string; children?: ReactNode }) => <a href={to}>{children}</a>,
}));

import { LandingTemplate } from './index';

describe('LandingTemplate', () => {
  it('composes header, hero, sections and footer', () => {
    const { container } = renderWithI18n(<LandingTemplate />, 'en-ie');
    expect(screen.getByRole('banner')).toBeInTheDocument();
    expect(screen.getByRole('main')).toBeInTheDocument();
    expect(screen.getByRole('contentinfo')).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      'The tender they already counted as theirs?',
    );
    for (const id of ['problem', 'agents', 'vision']) {
      expect(container.querySelector(`#${id}`), id).not.toBeNull();
    }
    expect(screen.getByText('Public school renovations')).toBeInTheDocument();
    expect(container.querySelector('#site-footer'), 'site-footer').not.toBeNull();
  });
});
