import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Reveal } from './index';

describe('Reveal', () => {
  it('renders its children', () => {
    render(
      <Reveal>
        <p>revealed content</p>
      </Reveal>,
    );
    expect(screen.getByText('revealed content')).toBeInTheDocument();
  });
});
