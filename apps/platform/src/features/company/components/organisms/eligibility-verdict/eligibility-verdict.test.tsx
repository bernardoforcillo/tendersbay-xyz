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
vi.mock('posthog-js/react', () => ({ usePostHog: () => ({ capture: vi.fn() }) }));

import type { AssessmentView } from '~/features/company/constants';
import { EligibilityVerdict } from './index';

const ASSESSMENT: AssessmentView = {
  verdict: 'no_go',
  evaluatedAt: '2026-08-08T09:00:00Z',
  blockingGapCount: 1,
  gaps: [
    {
      status: 'shortfall',
      blocking: true,
      held: 'OG1 classifica I',
      remedy: 'avvalimento',
      requirement: {
        id: 'r1',
        kind: 'soa',
        source: 'document_excerpt',
        requirementText: 'Attestazione SOA OG1 classifica III',
      },
    },
  ],
  caveats: ['documents_unread'],
};

describe('EligibilityVerdict', () => {
  // The product rule, made mechanical: the FACT must be readable before the
  // verdict. A verdict that arrives first trains people to trust it blindly or
  // to stop reading it.
  it('renders the lead fact before the verdict in DOM order', () => {
    render(<EligibilityVerdict assessment={ASSESSMENT} location="scheda_gara" />);
    const lead = screen.getByTestId('eligibility-lead');
    const verdict = screen.getByTestId('eligibility-verdict');
    // Node.DOCUMENT_POSITION_FOLLOWING === 4
    expect(lead.compareDocumentPosition(verdict) & 4).toBeTruthy();
  });

  it('leads with the published requirement text and what the dossier holds', () => {
    render(<EligibilityVerdict assessment={ASSESSMENT} location="scheda_gara" />);
    const lead = screen.getByTestId('eligibility-lead').textContent ?? '';
    expect(lead).toContain('Attestazione SOA OG1 classifica III');
    expect(lead).toContain('OG1 classifica I');
  });

  it('renders a remedy line for every gap that has one', () => {
    render(<EligibilityVerdict assessment={ASSESSMENT} location="scheda_gara" />);
    expect(screen.getByText('Borrow the requirement (avvalimento)')).toBeTruthy();
  });

  it('renders the caveats — silence about them reads as a clean bill', () => {
    render(<EligibilityVerdict assessment={ASSESSMENT} location="scheda_gara" />);
    expect(screen.getByText('Some of the buyer’s documents have not been read.')).toBeTruthy();
  });

  // Not "no gaps found": nothing was recorded, which is ignorance.
  it('says no requirement was recorded when there are no gaps', () => {
    render(
      <EligibilityVerdict
        assessment={{ verdict: 'insufficient_data', gaps: [], evaluatedAt: 'x' }}
        location="scheda_gara"
      />,
    );
    const lead = screen.getByTestId('eligibility-lead').textContent ?? '';
    expect(lead).toBe('No participation requirement has been recorded for this gara.');
  });

  it('announces itself politely so a re-check after an answer is heard', () => {
    render(<EligibilityVerdict assessment={ASSESSMENT} location="scheda_gara" />);
    expect(screen.getByRole('status')).toBeTruthy();
  });
});
