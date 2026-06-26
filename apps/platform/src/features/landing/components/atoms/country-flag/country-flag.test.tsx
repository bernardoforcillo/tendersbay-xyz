import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { CountryFlag } from './index';

function renderFlag(props: Partial<React.ComponentProps<typeof CountryFlag>> = {}) {
  return render(
    <CountryFlag
      code="IT"
      name="Italy"
      portal="Acquisti in Rete (MEPA)"
      available={false}
      statusLabel="Coming soon"
      {...props}
    />,
  );
}

describe('CountryFlag', () => {
  it('renders a button labelled with the country name and status', () => {
    renderFlag();
    expect(screen.getByRole('button', { name: 'Italy — Coming soon' })).toBeInTheDocument();
  });

  it('renders grayscale when not available', () => {
    const { container } = renderFlag({ available: false });
    expect(container.querySelector('.grayscale')).not.toBeNull();
  });

  it('renders in full color (no grayscale) when available', () => {
    const { container } = renderFlag({ available: true, statusLabel: 'Available' });
    expect(container.querySelector('.grayscale')).toBeNull();
  });

  it('reveals a card with the national portal on focus', async () => {
    const user = userEvent.setup();
    renderFlag();
    await user.tab();
    const card = await screen.findByRole('tooltip');
    expect(card).toHaveTextContent('Acquisti in Rete (MEPA)');
  });
});
