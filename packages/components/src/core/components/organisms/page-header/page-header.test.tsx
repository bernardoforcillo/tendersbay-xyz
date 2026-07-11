import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { PageHeader } from './index';

describe('PageHeader', () => {
  it('renders the serif title, subtitle, and actions', () => {
    render(
      <PageHeader
        title="Workbenches"
        subtitle="4 active"
        actions={<button type="button">New</button>}
      />,
    );
    const title = screen.getByRole('heading', { name: 'Workbenches' });
    expect(title.className).toContain('font-display');
    expect(screen.getByText('4 active')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'New' })).toBeInTheDocument();
  });

  it('renders leading content and children below the row', () => {
    render(
      <PageHeader leading={<span data-testid="toggle" />} title="T">
        <nav data-testid="tabs" />
      </PageHeader>,
    );
    expect(screen.getByTestId('toggle')).toBeInTheDocument();
    expect(screen.getByTestId('tabs')).toBeInTheDocument();
  });
});
