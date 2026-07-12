import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Banner } from './index';

describe('Banner', () => {
  it('renders an error banner with alert role and red tone classes', () => {
    render(<Banner tone="error">Something went wrong.</Banner>);
    const banner = screen.getByRole('alert');
    expect(banner).toHaveTextContent('Something went wrong.');
    expect(banner.className).toContain('border-red-200');
    expect(banner.className).toContain('bg-red-50');
    expect(banner.className).toContain('text-red-700');
  });

  it('renders a success banner with status role and brand tone classes', () => {
    render(<Banner tone="success">Saved.</Banner>);
    const banner = screen.getByRole('status');
    expect(banner).toHaveTextContent('Saved.');
    expect(banner.className).toContain('border-brand-200');
    expect(banner.className).toContain('bg-brand-50');
    expect(banner.className).toContain('text-brand-800');
  });
});
