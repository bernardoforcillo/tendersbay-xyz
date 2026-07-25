import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
}));

const captureMock = vi.fn();
vi.mock('posthog-js/react', () => ({ usePostHog: () => ({ capture: captureMock }) }));

import { MessageBubble } from './index';

function tenderFixture(overrides: Partial<TenderResult> = {}): TenderResult {
  return {
    $typeName: 'tender.v1.TenderResult',
    id: 't-1',
    title: 'Fornitura cestini intelligenti IoT',
    buyerName: 'Comune di Milano',
    status: 'open',
    procedureType: 'open',
    country: 'IT',
    cpv: '34928480',
    value: 250_000n,
    currency: 'EUR',
    publishedAt: '',
    deadline: '',
    relevanceScore: 0,
    source: 'ted',
    sourceRef: 'ref-1',
    sourceUrl: '',
    ...overrides,
  } as TenderResult;
}

describe('MessageBubble', () => {
  it('renders a TenderResultsTable with each tender title for a tender_results message', () => {
    renderWithI18n(
      <MessageBubble
        message={{
          id: 'msg-1',
          role: 'tender_results',
          content: '',
          createdAt: new Date().toISOString(),
          tenders: [tenderFixture(), tenderFixture({ id: 't-2', title: 'Raccolta rifiuti smart' })],
        }}
        isPendingChoice={false}
        onSubmitChoice={vi.fn()}
      />,
    );
    expect(screen.getByText('Fornitura cestini intelligenti IoT')).toBeInTheDocument();
    expect(screen.getByText('Raccolta rifiuti smart')).toBeInTheDocument();
  });

  it('still renders plain text content for a regular assistant message', () => {
    renderWithI18n(
      <MessageBubble
        message={{
          id: 'msg-2',
          role: 'assistant',
          content: 'Ciao, come posso aiutarti?',
          createdAt: new Date().toISOString(),
        }}
        isPendingChoice={false}
        onSubmitChoice={vi.fn()}
      />,
    );
    expect(screen.getByText('Ciao, come posso aiutarti?')).toBeInTheDocument();
  });

  it('fires chat_tender_card_opened when a tender row is activated', async () => {
    const user = userEvent.setup();
    renderWithI18n(
      <MessageBubble
        message={{
          id: 'm-1',
          role: 'tender_results',
          content: '',
          createdAt: '',
          tenders: [tenderFixture({ title: 'Cestini intelligenti' })],
        }}
        isPendingChoice={false}
        onSubmitChoice={vi.fn()}
      />,
    );

    await user.click(screen.getByText('Cestini intelligenti'));

    expect(captureMock).toHaveBeenCalledWith(
      'chat_tender_card_opened',
      expect.objectContaining({ location: 'explore_chat', fit_tier: 'none' }),
    );
  });
});
