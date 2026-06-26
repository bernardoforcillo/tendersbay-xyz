import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { FooterColumn } from './index';

describe('FooterColumn', () => {
  it('renders a labelled nav with one link per item', () => {
    render(
      <FooterColumn
        heading="Product"
        links={[
          { label: 'The agents', href: '#agents' },
          { label: 'Coverage', href: '#coverage' },
        ]}
      />,
    );
    expect(screen.getByRole('navigation', { name: 'Product' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Product' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Coverage' })).toHaveAttribute('href', '#coverage');
  });
});
