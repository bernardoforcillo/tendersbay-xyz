import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { AgentStep } from './index';

describe('AgentStep', () => {
  it('renders a zero-padded index, title and body', () => {
    render(<AgentStep index={1} icon="search" title="Find" body="Finds tenders." />);
    expect(screen.getByText('01')).toBeInTheDocument();
    expect(screen.getByText('Find')).toBeInTheDocument();
    expect(screen.getByText('Finds tenders.')).toBeInTheDocument();
  });
});
