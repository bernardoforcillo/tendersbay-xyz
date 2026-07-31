import type { CpvMatch } from '@tendersbay/proto/tender/v1/tender_pb';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

import { AppliedCpvChips } from './index';

function match(overrides: Partial<CpvMatch> = {}): CpvMatch {
  return {
    $typeName: 'tender.v1.CpvMatch',
    code: '90919200',
    label: 'Servizi di pulizia di uffici',
    language: 'it',
    score: 0.9,
    ...overrides,
  } as CpvMatch;
}

describe('AppliedCpvChips', () => {
  it('renders nothing when the server inferred no code', () => {
    const { container } = renderWithI18n(<AppliedCpvChips matches={[]} onRemove={vi.fn()} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('shows the code and the official label so the user can judge the inference', () => {
    renderWithI18n(<AppliedCpvChips matches={[match()]} onRemove={vi.fn()} />);
    expect(screen.getByText(/90919200/)).toBeInTheDocument();
    expect(screen.getByText(/Servizi di pulizia di uffici/)).toBeInTheDocument();
  });

  it('removes one code at a time', async () => {
    const onRemove = vi.fn();
    renderWithI18n(
      <AppliedCpvChips
        matches={[match(), match({ code: '90911200', label: 'Servizi di pulizia di edifici' })]}
        onRemove={onRemove}
      />,
    );
    await userEvent.click(screen.getByRole('button', { name: /90911200/ }));
    expect(onRemove).toHaveBeenCalledWith('90911200');
    expect(onRemove).toHaveBeenCalledTimes(1);
  });

  it('gives each remove button an accessible name that names its chip', async () => {
    // Several identical-looking buttons in a row are unusable with a screen
    // reader unless each says what it removes.
    renderWithI18n(<AppliedCpvChips matches={[match()]} onRemove={vi.fn()} />);
    const button = screen.getByRole('button', { name: /90919200/ });
    expect(button).toHaveAccessibleName();
  });
});
