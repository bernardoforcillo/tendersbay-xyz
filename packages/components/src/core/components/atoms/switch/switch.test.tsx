import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { Switch } from './index';

describe('Switch', () => {
  it('toggles selection on click and reports the new state', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Switch onChange={onChange}>Email alerts</Switch>);
    const control = screen.getByRole('switch', { name: 'Email alerts' });
    expect(control).not.toBeChecked();
    await user.click(control);
    expect(onChange).toHaveBeenCalledWith(true);
    expect(control).toBeChecked();
  });

  it('drives the track container with group-data-[selected] styling', () => {
    render(<Switch defaultSelected>Email alerts</Switch>);
    const root = screen.getByRole('switch', { name: 'Email alerts' }).closest('label');
    const track = root?.querySelector('div');
    expect(track?.className).toContain('group-data-[selected]:bg-brand-600');
  });

  it('renders the label text after the track', () => {
    render(<Switch>Push notifications</Switch>);
    expect(screen.getByText('Push notifications')).toBeInTheDocument();
  });
});
