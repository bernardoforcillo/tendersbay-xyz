import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ValueCard } from './index';

describe('ValueCard', () => {
  it('renders the title and body', () => {
    render(<ValueCard icon="search" title="Find" body="The agents find the right tender." />);
    expect(screen.getByText('Find')).toBeInTheDocument();
    expect(screen.getByText('The agents find the right tender.')).toBeInTheDocument();
  });
});
