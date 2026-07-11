import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { EmptyState } from './index';

describe('EmptyState', () => {
  it('renders title, description, and the action slot', () => {
    render(
      <EmptyState
        title="No workbenches yet"
        description="Create one to organize a tender."
        action={<button type="button">Create workbench</button>}
      />,
    );
    expect(screen.getByRole('heading', { name: 'No workbenches yet' })).toBeInTheDocument();
    expect(screen.getByText('Create one to organize a tender.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Create workbench' })).toBeInTheDocument();
  });

  it('hides the icon from the a11y tree', () => {
    render(<EmptyState icon={<svg data-testid="icon" />} title="Empty" />);
    expect(screen.getByTestId('icon').parentElement).toHaveAttribute('aria-hidden', 'true');
  });
});
