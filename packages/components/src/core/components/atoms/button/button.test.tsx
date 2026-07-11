import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { Button } from './index';

describe('Button', () => {
  it('renders its label and handles press', async () => {
    const user = userEvent.setup();
    const onPress = vi.fn();
    render(<Button onPress={onPress}>Save</Button>);
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(onPress).toHaveBeenCalledTimes(1);
  });

  it('applies the variant and size classes', () => {
    render(
      <Button variant="ghost" size="lg">
        Ghost
      </Button>,
    );
    const button = screen.getByRole('button', { name: 'Ghost' });
    expect(button.className).toContain('border-cream-300');
    expect(button.className).toContain('h-12');
  });

  it('defaults to the primary md 40px target', () => {
    render(<Button>Go</Button>);
    const button = screen.getByRole('button', { name: 'Go' });
    expect(button.className).toContain('bg-brand-600');
    expect(button.className).toContain('h-10');
  });

  it('lets a consumer className override the defaults', () => {
    render(<Button className="h-12">Big</Button>);
    const button = screen.getByRole('button', { name: 'Big' });
    expect(button.className).toContain('h-12');
    expect(button.className).not.toContain('h-10');
  });

  it('disables interaction with isDisabled', async () => {
    const user = userEvent.setup();
    const onPress = vi.fn();
    render(
      <Button isDisabled onPress={onPress}>
        Nope
      </Button>,
    );
    await user.click(screen.getByRole('button', { name: 'Nope' }));
    expect(onPress).not.toHaveBeenCalled();
  });
});
