import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (k: string, d?: string) => d ?? k, i18n: { language: 'en-ie' } }),
}));
vi.mock('@tanstack/react-router', () => ({ useNavigate: () => vi.fn() }));
vi.mock('posthog-js/react', () => ({ usePostHog: () => ({ capture: vi.fn() }) }));
vi.mock('~/features/workbench/components/molecules/workbench-picker', () => ({
  WorkbenchPicker: () => <div>picker</div>,
}));
vi.mock('~/features/account/components/organisms/tender-feed', () => ({
  deadlineInfo: () => null,
}));
vi.mock('~/lib/api/client', () => ({ bidClient: { addBid: vi.fn() } }));
vi.mock('~/store/auth', () => ({
  useAuthStore: (sel: (s: { isAuthenticated: boolean }) => unknown) =>
    sel({ isAuthenticated: true }),
}));
const wsState: { currentWorkspaceId: string | null } = { currentWorkspaceId: null };
vi.mock('~/store/workspace', () => ({
  useWorkspaceStore: (sel: (s: { currentWorkspaceId: string | null }) => unknown) => sel(wsState),
}));

import { PrepareInWorkbench } from './index';

describe('PrepareInWorkbench gating', () => {
  it('renders nothing without a current workspace', () => {
    wsState.currentWorkspaceId = null;
    const { container } = render(<PrepareInWorkbench tenderId="t1" deadline="" />);
    expect(container.firstChild).toBeNull();
  });

  it('renders the prepare button when a workspace is selected', () => {
    wsState.currentWorkspaceId = 'ws1';
    render(<PrepareInWorkbench tenderId="t1" deadline="" />);
    expect(screen.getByRole('button', { name: 'bid.actions.prepare' })).toBeTruthy();
  });
});
