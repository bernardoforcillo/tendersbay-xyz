import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';

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

const capture = vi.fn();
vi.mock('posthog-js/react', () => ({ usePostHog: () => ({ capture }) }));

const client = vi.hoisted(() => ({
  putSoaCategory: vi.fn().mockResolvedValue({}),
  putCertification: vi.fn().mockResolvedValue({}),
  putFinancialYear: vi.fn().mockResolvedValue({}),
  putPastContract: vi.fn().mockResolvedValue({}),
  putRegistration: vi.fn().mockResolvedValue({}),
}));
vi.mock('~/lib/api/client', () => ({ companyClient: client, tenderClient: {} }));

import type { RequirementView } from '~/features/company/constants';
import { CaptureQuestion } from './index';

function requirement(kind: string): RequirementView {
  return {
    id: `r-${kind}`,
    tenderId: '42',
    kind,
    source: 'document_excerpt',
    requirementText: 'Requisito pubblicato',
  };
}

function renderQuestion(kind: string, onSaved = vi.fn()) {
  return render(
    <CaptureQuestion
      workspaceId="ws1"
      tenderId="42"
      requirement={requirement(kind)}
      location="bid_detail"
      onSaved={onSaved}
    />,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  // Deterministic markup for the animated children.
  window.matchMedia = ((q: string) => ({
    matches: q.includes('reduce'),
    media: q,
    onchange: null,
    addEventListener() {},
    removeEventListener() {},
    addListener() {},
    removeListener() {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
});

describe('CaptureQuestion', () => {
  it('renders the published requirement text above the controls', () => {
    renderQuestion('soa');
    expect(screen.getByText('Requisito pubblicato')).toBeTruthy();
  });

  it('routes an SOA answer to PutSoaCategory', async () => {
    const user = userEvent.setup();
    renderQuestion('soa');
    await user.click(screen.getByRole('button', { name: 'OG1' }));
    await user.click(screen.getByRole('button', { name: 'III-bis' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(client.putSoaCategory).toHaveBeenCalledTimes(1);
    const arg = client.putSoaCategory.mock.calls[0]?.[0];
    expect(arg.soa.category).toBe('OG1');
    expect(arg.soa.classifica).toBe('III-bis');
  });

  it('routes a certification answer to PutCertification', async () => {
    const user = userEvent.setup();
    renderQuestion('certification');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(client.putCertification).toHaveBeenCalledTimes(1);
  });

  // Turnover and headcount share one FinancialYear record: Italian selection
  // criteria ask for both per esercizio.
  it('routes both turnover and headcount to PutFinancialYear', async () => {
    const user = userEvent.setup();
    const turnover = renderQuestion('turnover');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(client.putFinancialYear).toHaveBeenCalledTimes(1);
    turnover.unmount();

    renderQuestion('headcount');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(client.putFinancialYear).toHaveBeenCalledTimes(2);
  });

  it('routes a past-contract answer to PutPastContract', async () => {
    const user = userEvent.setup();
    renderQuestion('past_contracts');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(client.putPastContract).toHaveBeenCalledTimes(1);
  });

  it('routes a registration answer to PutRegistration', async () => {
    const user = userEvent.setup();
    renderQuestion('registration');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(client.putRegistration).toHaveBeenCalledTimes(1);
  });

  // The server derives provenance/statedBy/statedAt from how the call arrived and
  // ignores whatever a client sends, so a client that sent them would be claiming
  // a lever it does not have.
  it('sends prompted_by attribution and never provenance, statedBy or statedAt', async () => {
    const user = userEvent.setup();
    renderQuestion('soa');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    const attribution = client.putSoaCategory.mock.calls[0]?.[0].soa.attribution;
    expect(attribution.promptedBy).toBe('tender');
    expect(attribution.promptedByTenderId).toBe('42');
    expect(attribution).not.toHaveProperty('provenance');
    expect(attribution).not.toHaveProperty('statedBy');
    expect(attribution).not.toHaveProperty('statedAt');
  });

  it('writes nothing for an unclassified requirement and keeps its save focusable', async () => {
    const user = userEvent.setup();
    renderQuestion('other');
    const save = screen.getByRole('button', { name: 'Save' });
    // aria-disabled, NOT isDisabled: a disabled control leaves the tab order and
    // takes its explanation with it.
    expect(save.getAttribute('aria-disabled')).toBe('true');
    expect(save.hasAttribute('disabled')).toBe(false);
    await user.click(save);
    expect(client.putSoaCategory).not.toHaveBeenCalled();
  });

  // "Not sure" is not an answer. Nothing is written, so the question returns on
  // the next visit — `unknown` is what triggers the ask in the first place.
  it('collapses without writing when the user is not sure', async () => {
    const user = userEvent.setup();
    const onSaved = vi.fn();
    renderQuestion('soa', onSaved);
    await user.click(screen.getByRole('button', { name: 'Not sure' }));
    expect(screen.queryByText('Requisito pubblicato')).toBeNull();
    expect(client.putSoaCategory).not.toHaveBeenCalled();
    expect(onSaved).not.toHaveBeenCalled();
    expect(capture).toHaveBeenCalledWith('company_dossier_question_skipped', {
      location: 'bid_detail',
      field_category: 'soa',
    });
  });
});
