import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { CountryFlag } from './index';

describe('CountryFlag', () => {
  it('exposes the country name and status to assistive tech', () => {
    render(
      <ul>
        <CountryFlag code="IT" name="Italy" available={false} statusLabel="Coming soon" />
      </ul>,
    );
    expect(screen.getByText('Italy — Coming soon')).toBeInTheDocument();
  });

  it('renders grayscale when not available', () => {
    const { container } = render(
      <ul>
        <CountryFlag code="IT" name="Italy" available={false} statusLabel="Coming soon" />
      </ul>,
    );
    expect(container.querySelector('.grayscale')).not.toBeNull();
  });

  it('renders in full color (no grayscale) when available', () => {
    const { container } = render(
      <ul>
        <CountryFlag code="IT" name="Italy" available statusLabel="Available" />
      </ul>,
    );
    expect(container.querySelector('.grayscale')).toBeNull();
  });
});
