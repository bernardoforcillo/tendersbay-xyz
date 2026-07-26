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
      'Every public tender in 27 countries',
    );
    for (const id of ['problem', 'agents', 'vision']) {
      expect(container.querySelector(`#${id}`), id).not.toBeNull();
    }
    expect(screen.getByPlaceholderText('Public school renovations')).toBeInTheDocument();
    expect(container.querySelector('#site-footer'), 'site-footer').not.toBeNull();
  });

  it('places the coverage section before the assurance section', () => {
    const { container } = renderWithI18n(<LandingTemplate />, 'en-ie');
    const coverage = container.querySelector('#coverage');
    const assurance = container.querySelector('#assurance');
    expect(coverage, 'coverage section').not.toBeNull();
    expect(assurance, 'assurance section').not.toBeNull();
    // Coverage must come first: assurance follows it in document order.
    expect(
      coverage!.compareDocumentPosition(assurance!) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });
});
