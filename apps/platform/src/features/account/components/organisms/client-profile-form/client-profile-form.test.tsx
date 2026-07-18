import type { ClientProfile } from '@tendersbay/proto/workspace/v1/workspace_pb';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { renderWithI18n } from '~/test/utils';

const updateClientProfile = vi.fn();
vi.mock('~/lib/api/client', () => ({
  workspaceClient: { updateClientProfile: (...args: unknown[]) => updateClientProfile(...args) },
}));

const captureMock = vi.fn();
vi.mock('posthog-js/react', () => ({ usePostHog: () => ({ capture: captureMock }) }));

import { ClientProfileForm } from './index';

function fakeProfile(overrides: Partial<ClientProfile> = {}): ClientProfile {
  return {
    $typeName: 'workspace.v1.ClientProfile',
    workspaceId: 'ws-1',
    sectors: [],
    countries: [],
    regions: [],
    procedureTypes: [],
    valueMin: 0n,
    valueMinSet: false,
    valueMax: 0n,
    valueMaxSet: false,
    notes: '',
    updatedAt: '',
    ...overrides,
  };
}

describe('ClientProfileForm', () => {
  beforeEach(() => {
    updateClientProfile.mockReset();
    captureMock.mockReset();
  });

  it('submits the selected sectors/countries and calls onSaved with the server response', async () => {
    const saved = fakeProfile({ sectors: ['45'], countries: ['IT'] });
    updateClientProfile.mockResolvedValue({ profile: saved });
    const onSaved = vi.fn();

    renderWithI18n(<ClientProfileForm workspaceId="ws-1" onSaved={onSaved} />);

    fireEvent.click(screen.getByRole('button', { name: /construction/i }));
    fireEvent.click(screen.getByRole('button', { name: /italy/i }));
    fireEvent.click(screen.getByRole('button', { name: /save profile/i }));

    await waitFor(() => expect(updateClientProfile).toHaveBeenCalled());
    expect(updateClientProfile).toHaveBeenCalledWith(
      expect.objectContaining({
        workspaceId: 'ws-1',
        sectors: ['45'],
        countries: ['IT'],
        regions: [],
        procedureTypes: [],
      }),
    );
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(saved));
  });

  it('parses comma-separated free-text regions and submits toggled procedure types', async () => {
    updateClientProfile.mockResolvedValue({ profile: fakeProfile() });

    renderWithI18n(<ClientProfileForm workspaceId="ws-1" onSaved={vi.fn()} />);

    fireEvent.change(screen.getByLabelText(/regions/i), { target: { value: ' itc, de3 ' } });
    fireEvent.click(screen.getByRole('button', { name: /open procedure/i }));
    fireEvent.click(screen.getByRole('button', { name: /save profile/i }));

    await waitFor(() => expect(updateClientProfile).toHaveBeenCalled());
    expect(updateClientProfile).toHaveBeenCalledWith(
      expect.objectContaining({
        workspaceId: 'ws-1',
        regions: ['ITC', 'DE3'],
        procedureTypes: ['open'],
      }),
    );
  });

  it('toggles procedure-type chips on and off, the same interaction as sectors/countries', async () => {
    renderWithI18n(<ClientProfileForm workspaceId="ws-1" onSaved={vi.fn()} />);

    const restricted = screen.getByRole('button', { name: /restricted procedure/i });
    expect(restricted).toHaveAttribute('aria-pressed', 'false');

    fireEvent.click(restricted);
    expect(restricted).toHaveAttribute('aria-pressed', 'true');

    fireEvent.click(restricted);
    expect(restricted).toHaveAttribute('aria-pressed', 'false');
  });

  it('fires client_profile_completed with consent-safe bands, including region_count and has_procedure_filter', async () => {
    updateClientProfile.mockResolvedValue({
      profile: fakeProfile({ sectors: ['45'], countries: ['IT'] }),
    });

    renderWithI18n(<ClientProfileForm workspaceId="ws-1" onSaved={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /construction/i }));
    fireEvent.click(screen.getByRole('button', { name: /italy/i }));
    fireEvent.change(screen.getByLabelText(/regions/i), { target: { value: 'itc, de3' } });
    fireEvent.click(screen.getByRole('button', { name: /open procedure/i }));
    fireEvent.click(screen.getByRole('button', { name: /save profile/i }));

    await waitFor(() => expect(captureMock).toHaveBeenCalled());
    expect(captureMock).toHaveBeenCalledWith('client_profile_completed', {
      location: 'explore_profile_form',
      sector_count: '1',
      country_count: '1',
      region_count: '2-3',
      has_value_band: false,
      has_procedure_filter: true,
    });
  });

  it('tags the completion event with a caller-supplied location, overriding the default', async () => {
    updateClientProfile.mockResolvedValue({ profile: fakeProfile() });

    renderWithI18n(<ClientProfileForm workspaceId="ws-1" onSaved={vi.fn()} location="first_run" />);
    fireEvent.click(screen.getByRole('button', { name: /save profile/i }));

    await waitFor(() => expect(captureMock).toHaveBeenCalled());
    expect(captureMock).toHaveBeenCalledWith(
      'client_profile_completed',
      expect.objectContaining({ location: 'first_run' }),
    );
  });

  it('reports zero/false bands when regions and procedure types are left as "any"', async () => {
    updateClientProfile.mockResolvedValue({ profile: fakeProfile() });

    renderWithI18n(<ClientProfileForm workspaceId="ws-1" onSaved={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /save profile/i }));

    await waitFor(() => expect(captureMock).toHaveBeenCalled());
    expect(captureMock).toHaveBeenCalledWith('client_profile_completed', {
      location: 'explore_profile_form',
      sector_count: '0',
      country_count: '0',
      region_count: '0',
      has_value_band: false,
      has_procedure_filter: false,
    });
  });

  it('blocks submission with an inverted value band, without calling the server', async () => {
    renderWithI18n(<ClientProfileForm workspaceId="ws-1" onSaved={vi.fn()} />);

    fireEvent.change(screen.getByLabelText(/minimum value/i), { target: { value: '500' } });
    fireEvent.change(screen.getByLabelText(/maximum value/i), { target: { value: '100' } });
    fireEvent.click(screen.getByRole('button', { name: /save profile/i }));

    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(updateClientProfile).not.toHaveBeenCalled();
  });

  it('pre-fills from the initial profile, including regions and procedure types', () => {
    renderWithI18n(
      <ClientProfileForm
        workspaceId="ws-1"
        onSaved={vi.fn()}
        initial={fakeProfile({
          sectors: ['45'],
          countries: ['IT'],
          regions: ['ITC'],
          procedureTypes: ['open'],
          notes: 'renovation work',
        })}
      />,
    );
    expect(screen.getByRole('button', { name: /construction/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('button', { name: /italy/i })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: /open procedure/i })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByDisplayValue('ITC')).toBeInTheDocument();
    expect(screen.getByDisplayValue('renovation work')).toBeInTheDocument();
  });
});
