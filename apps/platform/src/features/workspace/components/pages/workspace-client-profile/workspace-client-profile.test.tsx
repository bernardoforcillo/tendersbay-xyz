import { fireEvent, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

const getClientProfile = vi.fn();
vi.mock('~/lib/api/client', () => ({
  workspaceClient: { getClientProfile: (...args: unknown[]) => getClientProfile(...args) },
}));

vi.mock('~/features/workspace/context', () => ({
  useWorkspaceContext: () => ({ workspaceId: 'ws-1' }),
}));

vi.mock('~/features/account/components/organisms', () => ({
  // biome-ignore lint/suspicious/noExplicitAny: test stub, props are read only for assertions
  ClientProfileForm: (props: any) => (
    <div
      data-testid="client-profile-form"
      data-location={props.location}
      data-notes={props.initial?.notes ?? ''}
    >
      <button type="button" onClick={() => props.onSaved({ notes: 'updated notes' })}>
        save
      </button>
    </div>
  ),
}));

import { WorkspaceClientProfilePage } from './index';

describe('WorkspaceClientProfilePage', () => {
  beforeEach(() => {
    getClientProfile.mockReset();
  });

  it('shows loading, then the empty form when no profile exists yet', async () => {
    getClientProfile.mockResolvedValue({ exists: false, profile: undefined });
    renderWithI18n(<WorkspaceClientProfilePage />);

    expect(screen.getByText(/loading/i)).toBeInTheDocument();
    const form = await screen.findByTestId('client-profile-form');
    expect(form).toHaveAttribute('data-location', 'settings');
    expect(form).toHaveAttribute('data-notes', '');
    expect(getClientProfile).toHaveBeenCalledWith({ workspaceId: 'ws-1' });
  });

  it('pre-fills the form from an existing profile', async () => {
    getClientProfile.mockResolvedValue({ exists: true, profile: { notes: 'renovation work' } });
    renderWithI18n(<WorkspaceClientProfilePage />);

    const form = await screen.findByTestId('client-profile-form');
    expect(form).toHaveAttribute('data-notes', 'renovation work');
  });

  it('shows a saved confirmation after the form reports a save', async () => {
    getClientProfile.mockResolvedValue({ exists: false, profile: undefined });
    renderWithI18n(<WorkspaceClientProfilePage />);

    await screen.findByTestId('client-profile-form');
    fireEvent.click(screen.getByRole('button', { name: 'save' }));

    expect(await screen.findByText(/saved/i)).toBeInTheDocument();
  });

  it('shows an error banner when the profile fetch fails', async () => {
    getClientProfile.mockRejectedValue(new Error('boom'));
    renderWithI18n(<WorkspaceClientProfilePage />);

    expect(await screen.findByRole('alert')).toHaveTextContent('boom');
  });
});
