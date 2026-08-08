import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opt?: string | Record<string, unknown>) => {
      if (typeof opt === 'string') return opt;
      const template = (opt?.defaultValue as string) ?? key;
      return template.replace(/{{(\w+)}}/g, (_m, name: string) => String(opt?.[name] ?? ''));
    },
    i18n: { language: 'en-ie' },
  }),
}));

import { CoverageBadge } from './index';

describe('CoverageBadge', () => {
  it('renders the quantity and the cause as two separate lines', () => {
    const { container } = render(
      <CoverageBadge coverage="notice_only" reason="body_not_retrieved" />,
    );
    expect(screen.getByText('Notice only')).toBeTruthy();
    expect(
      screen.getByText('The buyer publishes documents we have not retrieved yet.'),
    ).toBeTruthy();
    // Two nodes, not one concatenated string: the quantity and the cause are
    // orthogonal facts and must stay separately readable.
    expect(container.querySelectorAll('p').length).toBe(1);
  });

  it('never renders the full-coverage label for partial coverage', () => {
    render(<CoverageBadge coverage="notice_only" reason="notice_not_read" />);
    expect(screen.queryByText('Documents read')).toBeNull();
  });

  it('never renders the full-coverage label for no coverage', () => {
    render(<CoverageBadge coverage="none" reason="extraction_failed" />);
    expect(screen.queryByText('Documents read')).toBeNull();
    expect(screen.getByText('Nothing read')).toBeTruthy();
  });

  // The one string in the product that asserts absence. Every other cause
  // describes a limit of our own work, not of what the buyer published.
  it('is the only reason whose copy asserts the buyer published nothing', () => {
    const absence = 'The buyer published no documents with this notice.';
    const published = render(
      <CoverageBadge coverage="notice_only" reason="no_documents_published" />,
    );
    expect(screen.getByText(absence)).toBeTruthy();
    published.unmount();

    for (const reason of [
      'notice_not_read',
      'body_not_retrieved',
      'not_yet_extracted',
      'extraction_failed',
    ] as const) {
      const other = render(<CoverageBadge coverage="notice_only" reason={reason} />);
      expect(screen.queryByText(absence), reason).toBeNull();
      other.unmount();
    }
  });

  it('omits the cause line entirely when coverage is full', () => {
    render(<CoverageBadge coverage="full" reason="" />);
    expect(screen.getByText('Documents read')).toBeTruthy();
    expect(screen.queryByText(/buyer/i)).toBeNull();
  });
});
