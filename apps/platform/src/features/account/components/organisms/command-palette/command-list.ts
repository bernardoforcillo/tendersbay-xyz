import type { RecentWorkbench } from '~/store/recent-workbenches';

export type PaletteCommand = {
  id: string;
  section: 'navigate' | 'recent' | 'workspaces';
  label: string;
  to: string;
  params?: Record<string, string>;
};

type BuildInput = {
  workspaceId: string | null;
  recent: RecentWorkbench[];
  workspaces: { id: string; name: string }[];
  labels: {
    today: string;
    explore: string;
    workbenches: string;
    workspaceSettings: string;
    accountSettings: string;
    allWorkspaces: string;
  };
};

export function buildCommands({ workspaceId, recent, workspaces, labels }: BuildInput) {
  const commands: PaletteCommand[] = [];
  if (workspaceId) {
    commands.push({
      id: 'nav-today',
      section: 'navigate',
      label: labels.today,
      to: '/workspaces/$workspaceId',
      params: { workspaceId },
    });
  }
  commands.push({ id: 'nav-explore', section: 'navigate', label: labels.explore, to: '/explore' });
  if (workspaceId) {
    commands.push({
      id: 'nav-workbenches',
      section: 'navigate',
      label: labels.workbenches,
      to: '/workspaces/$workspaceId/workbenches',
      params: { workspaceId },
    });
    commands.push({
      id: 'nav-workspace-settings',
      section: 'navigate',
      label: labels.workspaceSettings,
      to: '/workspaces/$workspaceId/settings',
      params: { workspaceId },
    });
  }
  commands.push({
    id: 'nav-account-settings',
    section: 'navigate',
    label: labels.accountSettings,
    to: '/settings',
  });
  commands.push({
    id: 'nav-all-workspaces',
    section: 'navigate',
    label: labels.allWorkspaces,
    to: '/workspaces',
  });
  for (const r of recent) {
    commands.push({
      id: `wb-${r.workbenchId}`,
      section: 'recent',
      label: r.name,
      to: '/workspaces/$workspaceId/workbench/$workbenchId',
      params: { workspaceId: r.workspaceId, workbenchId: r.workbenchId },
    });
  }
  for (const w of workspaces) {
    commands.push({
      id: `ws-${w.id}`,
      section: 'workspaces',
      label: w.name,
      to: '/workspaces/$workspaceId',
      params: { workspaceId: w.id },
    });
  }
  return commands;
}
