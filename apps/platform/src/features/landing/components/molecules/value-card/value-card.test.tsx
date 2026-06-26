import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ValueCard } from './index';

describe('ValueCard', () => {
  it('renders the title and body', () => {
    render(<ValueCard icon="search" title="Find" body="The agents find the right tender." />);
    expect(screen.getByText('Find')).toBeInTheDocument();
    expect(screen.getByText('The agents find the right tender.')).toBeInTheDocument();
  });

  it('uses a muted cream surface when tone is "muted"', () => {
    const { container } = render(
      <ValueCard icon="map" title="Scattered" body="Across portals." tone="muted" />,
    );
    const card = container.firstElementChild as HTMLElement;
    expect(card.className).toContain('bg-cream-50');
  });

  it('defaults to the white solution surface', () => {
    const { container } = render(<ValueCard icon="search" title="Find" body="Finds." />);
    const card = container.firstElementChild as HTMLElement;
    expect(card.className).toContain('bg-white');
  });
});
