import { describe, expect, it } from 'vitest';
import { buildCommands } from './command-list';

const labels = {
  today: 'Today',
  explore: 'Explore',
  workbenches: 'Workbenches',
  workspaceSettings: 'Workspace settings',
  accountSettings: 'Account settings',
  allWorkspaces: 'All workspaces',
};

describe('buildCommands', () => {
  it('includes workspace-scoped destinations only with an active workspace', () => {
    const withWs = buildCommands({ workspaceId: 'ws-1', recent: [], workspaces: [], labels });
    expect(withWs.map((c) => c.id)).toEqual([
      'nav-today',
      'nav-explore',
      'nav-workbenches',
      'nav-workspace-settings',
      'nav-account-settings',
      'nav-all-workspaces',
    ]);
    expect(withWs[0]).toMatchObject({
      to: '/workspaces/$workspaceId',
      params: { workspaceId: 'ws-1' },
    });

    const withoutWs = buildCommands({ workspaceId: null, recent: [], workspaces: [], labels });
    expect(withoutWs.map((c) => c.id)).toEqual([
      'nav-explore',
      'nav-account-settings',
      'nav-all-workspaces',
    ]);
  });

  it('maps recent workbenches to their route with both params', () => {
    const commands = buildCommands({
      workspaceId: 'ws-1',
      recent: [{ workbenchId: 'wb-9', workspaceId: 'ws-2', name: 'Gara Comune MI', visitedAt: 1 }],
      workspaces: [],
      labels,
    });
    const wb = commands.find((c) => c.id === 'wb-wb-9');
    expect(wb).toMatchObject({
      section: 'recent',
      label: 'Gara Comune MI',
      to: '/workspaces/$workspaceId/workbench/$workbenchId',
      params: { workspaceId: 'ws-2', workbenchId: 'wb-9' },
    });
  });

  it('maps workspaces to switch targets', () => {
    const commands = buildCommands({
      workspaceId: null,
      recent: [],
      workspaces: [{ id: 'ws-3', name: 'ACME' }],
      labels,
    });
    const ws = commands.find((c) => c.id === 'ws-ws-3');
    expect(ws).toMatchObject({
      section: 'workspaces',
      label: 'ACME',
      to: '/workspaces/$workspaceId',
      params: { workspaceId: 'ws-3' },
    });
  });
});
