import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Pill } from './index';

describe('Pill', () => {
  it('renders neutral by default', () => {
    render(<Pill>3 open</Pill>);
    expect(screen.getByText('3 open').className).toContain('bg-cream-200');
  });

  it.each([
    ['match', 'bg-brand-100'],
    ['deadline', 'bg-signal-warm-100'],
    ['urgent', 'bg-signal-urgent-100'],
  ] as const)('tone %s applies its fixed semantic color', (tone, expected) => {
    render(<Pill tone={tone}>x</Pill>);
    expect(screen.getByText('x').className).toContain(expected);
  });
});
