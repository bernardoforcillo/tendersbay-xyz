import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderWithI18n } from '~/test/utils';
import { type Tender, TenderCard } from './index';

const tender: Tender = {
  id: 'lis-it',
  entity: 'Câmara de Lisboa',
  object: 'Fornecimento de serviços de TI',
  value: '€ 240.000',
  deadlineDays: 12,
  scoutCount: 12,
};

describe('TenderCard', () => {
  it('renders the tender entity, object and translated deadline', () => {
    renderWithI18n(<TenderCard tender={tender} />, 'en-ie');
    expect(screen.getByText('Câmara de Lisboa')).toBeInTheDocument();
    expect(screen.getByText('Fornecimento de serviços de TI')).toBeInTheDocument();
    expect(screen.getByText('12 days')).toBeInTheDocument();
  });
});
