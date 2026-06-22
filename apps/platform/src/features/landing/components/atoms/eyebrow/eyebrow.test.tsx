import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Eyebrow } from './index';

describe('Eyebrow', () => {
  it('renders its label', () => {
    render(<Eyebrow icon="sparkle">An AI agent team</Eyebrow>);
    expect(screen.getByText('An AI agent team')).toBeInTheDocument();
  });
});
