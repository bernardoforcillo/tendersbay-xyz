import { fireEvent, render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { Button } from './index';

vi.mock('@tanstack/react-router', () => ({
  Link: ({
    to,
    children,
    onClick,
    className,
  }: {
    to: string;
    children?: ReactNode;
    onClick?: () => void;
    className?: string;
  }) => (
    <a href={to} onClick={onClick} className={className}>
      {children}
    </a>
  ),
}));

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

  it('renders a router link and fires onPress on click', () => {
    const onPress = vi.fn();
    render(
      <Button to="/en-ie/auth/signup" search={{ entry: 'hero' }} onPress={onPress} variant="text">
        Put your agents to work
      </Button>,
    );
    const link = screen.getByRole('link', { name: 'Put your agents to work' });
    expect(link).toHaveAttribute('href', '/en-ie/auth/signup');
    fireEvent.click(link);
    expect(onPress).toHaveBeenCalledTimes(1);
  });
});
