import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Skeleton } from './index';

describe('Skeleton', () => {
  it('renders with text variant by default', () => {
    const { container } = render(<Skeleton className="w-48 h-4" />);
    const el = container.firstElementChild as HTMLElement;
    expect(el.className).toContain('animate-pulse');
    expect(el.className).toContain('bg-cream-200');
    expect(el.className).toContain('rounded-md');
    expect(el).toHaveAttribute('aria-hidden', 'true');
  });

  it('applies circle variant', () => {
    const { container } = render(<Skeleton variant="circle" className="w-10 h-10" />);
    expect((container.firstElementChild as HTMLElement).className).toContain('rounded-full');
  });

  it('applies rect variant', () => {
    const { container } = render(<Skeleton variant="rect" className="w-full h-24" />);
    expect((container.firstElementChild as HTMLElement).className).toContain('rounded-xl');
  });
});
