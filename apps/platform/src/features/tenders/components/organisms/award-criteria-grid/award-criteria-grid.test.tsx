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

import type { AwardCriterionView } from '~/features/tenders/constants';
import { AwardCriteriaGrid } from './index';

const NOT_ENRICHED = 'We have not read this notice’s structured data yet.';
const GRID_UNUSABLE =
  'The notice names the award method but publishes no weights. The scoring grid is in the disciplinare.';

function criterion(over: Partial<AwardCriterionView> = {}): AwardCriterionView {
  return { lotRef: '', ordinal: 1, type: 'quality', name: 'Technical merit', ...over };
}

describe('AwardCriteriaGrid', () => {
  // A zero weight and an absent weight are different published facts. Flattening
  // them overstates real coverage by roughly 2x.
  it('renders no weight cell and no zero when the weight is unpublished', () => {
    render(
      <AwardCriteriaGrid
        tender={{
          criteria: [criterion({ weightSet: false, weight: 0 })],
          gridUsableSet: true,
          gridUsable: true,
        }}
        location="scheda_gara"
      />,
    );
    expect(screen.getByText('No weight published')).toBeTruthy();
    expect(screen.queryByText('0')).toBeNull();
  });

  it('renders the published form beside the parsed weight when they differ', () => {
    render(
      <AwardCriteriaGrid
        tender={{
          criteria: [criterion({ weight: 30, weightSet: true, weightRaw: '30%' })],
          gridUsableSet: true,
          gridUsable: true,
        }}
        location="scheda_gara"
      />,
    );
    expect(screen.getByText('30')).toBeTruthy();
    expect(screen.getByText('(30%)')).toBeTruthy();
  });

  it('collapses per-lot grids that publish identical criteria into one block', () => {
    const shared = (lotRef: string) => [
      criterion({ lotRef, ordinal: 1, name: 'Technical merit', weight: 70, weightSet: true }),
      criterion({ lotRef, ordinal: 2, type: 'price', name: 'Price', weight: 30, weightSet: true }),
    ];
    render(
      <AwardCriteriaGrid
        tender={{
          criteria: [...shared('LOT-1'), ...shared('LOT-2'), ...shared('LOT-3')],
          gridUsableSet: true,
          gridUsable: true,
        }}
        location="scheda_gara"
      />,
    );
    expect(screen.getAllByText('Applies to every lot')).toHaveLength(1);
    expect(screen.getAllByText('Technical merit')).toHaveLength(1);
  });

  it('keeps per-lot grids separate when the lots publish different criteria', () => {
    render(
      <AwardCriteriaGrid
        tender={{
          criteria: [
            criterion({ lotRef: 'LOT-1', name: 'Technical merit', weight: 70, weightSet: true }),
            criterion({ lotRef: 'LOT-2', name: 'Delivery time', weight: 40, weightSet: true }),
          ],
          gridUsableSet: true,
          gridUsable: true,
        }}
        location="scheda_gara"
      />,
    );
    expect(screen.getByText('Lot LOT-1')).toBeTruthy();
    expect(screen.getByText('Lot LOT-2')).toBeTruthy();
  });

  // A multilingual notice repeats every text field per language; rendering the
  // raw list would show the same table two or three times over.
  it('shows one language when the notice publishes several', () => {
    render(
      <AwardCriteriaGrid
        tender={{
          criteria: [
            criterion({ name: 'Merito tecnico', lang: 'ITA' }),
            criterion({ name: 'Technical merit', lang: 'ENG' }),
          ],
          gridUsableSet: true,
          gridUsable: true,
        }}
        location="scheda_gara"
      />,
    );
    expect(screen.getByText('Technical merit')).toBeTruthy();
    expect(screen.queryByText('Merito tecnico')).toBeNull();
  });

  describe('the three empty-ish states render three different strings', () => {
    it('says the grid is in the disciplinare when the notice publishes no weights', () => {
      render(
        <AwardCriteriaGrid
          tender={{
            criteria: [],
            gridUsableSet: true,
            gridUsable: false,
            enrichedAt: '2026-08-01T00:00:00Z',
          }}
          location="scheda_gara"
        />,
      );
      expect(screen.getByText(GRID_UNUSABLE)).toBeTruthy();
      expect(screen.queryByText(NOT_ENRICHED)).toBeNull();
    });

    it('says the notice was never read when it was never enriched', () => {
      render(
        <AwardCriteriaGrid tender={{ criteria: [], enrichedAt: '' }} location="scheda_gara" />,
      );
      expect(screen.getByText(NOT_ENRICHED)).toBeTruthy();
      expect(screen.queryByText(GRID_UNUSABLE)).toBeNull();
    });

    it('never claims the gara has no criteria', () => {
      render(<AwardCriteriaGrid tender={{ criteria: [] }} location="scheda_gara" />);
      expect(screen.queryByText(/no criteria/i)).toBeNull();
    });
  });
});
