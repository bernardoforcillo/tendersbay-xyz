import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Icon } from './index';

describe('Icon', () => {
  it('renders the github glyph as an aria-hidden svg', () => {
    const { container } = render(<Icon name="github" />);
    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveAttribute('aria-hidden', 'true');
  });

  it('renders the linkedin and twitter glyphs', () => {
    const linkedin = render(<Icon name="linkedin" />).container.querySelector('svg');
    const twitter = render(<Icon name="twitter" />).container.querySelector('svg');
    expect(linkedin).toBeInTheDocument();
    expect(twitter).toBeInTheDocument();
  });
});
