import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Button } from './index';

describe('Button', () => {
  it('renders an anchor link to the target', () => {
    render(<Button href="#agents">See how it works</Button>);
    const link = screen.getByRole('link', { name: 'See how it works' });
    expect(link).toHaveAttribute('href', '#agents');
  });

  it('renders the ghost variant as a link too', () => {
    render(
      <Button href="#vision" variant="text">
        See the vision
      </Button>,
    );
    expect(screen.getByRole('link', { name: 'See the vision' })).toHaveAttribute('href', '#vision');
  });
});
