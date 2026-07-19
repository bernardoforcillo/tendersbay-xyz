import type { TenderResult } from '@tendersbay/proto/tender/v1/tender_pb';
import { screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

vi.mock('~/features/tenders', () => ({
  useTenderLink:
    () => (id: string, children: ReactNode, _className?: string, onClick?: () => void) => (
      <a href={`/tenders/${id}`} onClick={onClick}>
        {children}
      </a>
    ),
}));

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
  it('renders one TenderResultCard per tender for a tender_results message', () => {
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
});
