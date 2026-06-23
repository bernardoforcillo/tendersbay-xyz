import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Icon } from './index';

describe('Icon', () => {
  it('renders a decorative svg for the given name', () => {
    const { container } = render(<Icon name="search" />);
    const svg = container.querySelector('svg');
    expect(svg).not.toBeNull();
    expect(svg).toHaveAttribute('aria-hidden', 'true');
  });
});
