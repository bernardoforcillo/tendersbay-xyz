import { fireEvent, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LANDING_CARRY_OVER_KEY } from '~/lib/landing-carry-over';
import { renderWithI18n } from '~/test/utils';

const getClientProfile = vi.fn();
const updateClientProfile = vi.fn();
vi.mock('~/lib/api/client', () => ({
  workspaceClient: {
    getClientProfile: (...args: unknown[]) => getClientProfile(...args),
    updateClientProfile: (...args: unknown[]) => updateClientProfile(...args),
  },
}));

vi.mock('~/features/account/components/organisms', () => ({
  // biome-ignore lint/suspicious/noExplicitAny: test stub, props are read only for assertions
  ClientProfileForm: (props: any) => (
    <div
      data-testid="client-profile-form"
      data-location={props.location}
      data-notes={props.initial?.notes ?? ''}
    >
      <button type="button" onClick={() => props.onSaved({ notes: 'saved' })}>
        save
      </button>
    </div>
  ),
}));

import { useFirstRunStore } from '~/store/first-run';
import { FirstRunProfile } from './index';

describe('FirstRunProfile', () => {
  beforeEach(() => {
    getClientProfile.mockReset();
    updateClientProfile.mockReset();
    sessionStorage.clear();
    useFirstRunStore.setState({ skippedWorkspaceIds: [] });
  });

  it('renders ClientProfileForm when no profile exists yet', async () => {
    getClientProfile.mockResolvedValue({ exists: false, profile: undefined });

    renderWithI18n(
      <FirstRunProfile workspaceId="ws-1">
        <div data-testid="children" />
      </FirstRunProfile>,
    );

    expect(await screen.findByTestId('client-profile-form')).toBeInTheDocument();
    expect(screen.getByTestId('client-profile-form')).toHaveAttribute('data-location', 'first_run');
    expect(screen.queryByTestId('children')).not.toBeInTheDocument();
    expect(getClientProfile).toHaveBeenCalledWith({ workspaceId: 'ws-1' });
  });

  it('renders nothing (children pass through) when a profile already exists', async () => {
    getClientProfile.mockResolvedValue({ exists: true, profile: { notes: 'existing' } });

    renderWithI18n(
      <FirstRunProfile workspaceId="ws-1">
        <div data-testid="children" />
      </FirstRunProfile>,
    );

    expect(await screen.findByTestId('children')).toBeInTheDocument();
    expect(screen.queryByTestId('client-profile-form')).not.toBeInTheDocument();
  });

  it('renders nothing (children pass through) without a network call when the workspace was already skipped', async () => {
    useFirstRunStore.getState().skipWorkspace('ws-1');

    renderWithI18n(
      <FirstRunProfile workspaceId="ws-1">
        <div data-testid="children" />
      </FirstRunProfile>,
    );

    expect(screen.getByTestId('children')).toBeInTheDocument();
    expect(screen.queryByTestId('client-profile-form')).not.toBeInTheDocument();
    await waitFor(() => expect(getClientProfile).not.toHaveBeenCalled());
  });

  it('Skip records the workspace in the store and dismisses without writing to the server', async () => {
    getClientProfile.mockResolvedValue({ exists: false, profile: undefined });

    renderWithI18n(
      <FirstRunProfile workspaceId="ws-1">
        <div data-testid="children" />
      </FirstRunProfile>,
    );

    await screen.findByTestId('client-profile-form');
    fireEvent.click(screen.getByRole('button', { name: /skip/i }));

    expect(useFirstRunStore.getState().hasSkipped('ws-1')).toBe(true);
    expect(screen.getByTestId('children')).toBeInTheDocument();
    expect(screen.queryByTestId('client-profile-form')).not.toBeInTheDocument();
    expect(updateClientProfile).not.toHaveBeenCalled();
  });

  it('pre-fills the notes field from a pre-seeded landing carry-over, then clears it', async () => {
    sessionStorage.setItem(
      LANDING_CARRY_OVER_KEY,
      JSON.stringify({ query: 'road resurfacing works', filters: {} }),
    );
    getClientProfile.mockResolvedValue({ exists: false, profile: undefined });

    renderWithI18n(
      <FirstRunProfile workspaceId="ws-1">
        <div data-testid="children" />
      </FirstRunProfile>,
    );

    const form = await screen.findByTestId('client-profile-form');
    expect(form).toHaveAttribute('data-notes', 'road resurfacing works');
    expect(sessionStorage.getItem(LANDING_CARRY_OVER_KEY)).toBeNull();
  });

  it('clears a pre-seeded landing carry-over even when a profile already exists', async () => {
    sessionStorage.setItem(
      LANDING_CARRY_OVER_KEY,
      JSON.stringify({ query: 'road resurfacing works', filters: {} }),
    );
    getClientProfile.mockResolvedValue({ exists: true, profile: { notes: 'existing' } });

    renderWithI18n(
      <FirstRunProfile workspaceId="ws-1">
        <div data-testid="children" />
      </FirstRunProfile>,
    );

    expect(await screen.findByTestId('children')).toBeInTheDocument();
    expect(sessionStorage.getItem(LANDING_CARRY_OVER_KEY)).toBeNull();
  });

  it('clears a pre-seeded landing carry-over when the workspace was already skipped, without a network call', async () => {
    sessionStorage.setItem(
      LANDING_CARRY_OVER_KEY,
      JSON.stringify({ query: 'road resurfacing works', filters: {} }),
    );
    useFirstRunStore.getState().skipWorkspace('ws-1');

    renderWithI18n(
      <FirstRunProfile workspaceId="ws-1">
        <div data-testid="children" />
      </FirstRunProfile>,
    );

    expect(screen.getByTestId('children')).toBeInTheDocument();
    expect(sessionStorage.getItem(LANDING_CARRY_OVER_KEY)).toBeNull();
    await waitFor(() => expect(getClientProfile).not.toHaveBeenCalled());
  });

  it('fails open (renders children) if the profile check errors', async () => {
    getClientProfile.mockRejectedValue(new Error('network down'));

    renderWithI18n(
      <FirstRunProfile workspaceId="ws-1">
        <div data-testid="children" />
      </FirstRunProfile>,
    );

    expect(await screen.findByTestId('children')).toBeInTheDocument();
    expect(screen.queryByTestId('client-profile-form')).not.toBeInTheDocument();
  });
});
