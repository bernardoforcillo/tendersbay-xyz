import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Card } from './index';

describe('Card', () => {
  it('renders a soft white surface with default padding', () => {
    render(<Card data-testid="card">content</Card>);
    const card = screen.getByTestId('card');
    expect(card.className).toContain('bg-white');
    expect(card.className).toContain('shadow-soft');
    expect(card.className).toContain('p-5');
  });

  it('drops padding with padding="none"', () => {
    render(
      <Card data-testid="card" padding="none">
        x
      </Card>,
    );
    expect(screen.getByTestId('card').className).not.toContain('p-5');
  });
});
