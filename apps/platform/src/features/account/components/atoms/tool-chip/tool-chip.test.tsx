import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, opts?: { defaultValue?: string }) => opts?.defaultValue ?? _key,
  }),
}));

import { ToolChip } from './index';

describe('ToolChip', () => {
  it('renders the running label for a known tool', () => {
    render(<ToolChip name="search_tenders" status="running" />);
    expect(screen.getByText('Cerco bandi…')).toBeInTheDocument();
  });

  it('renders the done label with a check for a finished tool', () => {
    render(<ToolChip name="create_workbench" status="done" />);
    expect(screen.getByText(/Workbench creato/)).toBeInTheDocument();
  });

  it('falls back to the raw tool name for an unknown tool', () => {
    render(<ToolChip name="mystery_tool" status="running" />);
    expect(screen.getByText(/mystery_tool/)).toBeInTheDocument();
  });
});
