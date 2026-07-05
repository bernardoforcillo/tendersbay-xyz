import { PageHeader, SearchDock } from '~/features/account/components/organisms';
import { useWorkspaceContext } from '~/features/workspace/context';

export function WorkspaceOverviewPage() {
  const { workspace } = useWorkspaceContext();
  return (
    // Dock pinned to the bottom — animates to center on Explore via its shared layoutId.
    <div className="flex min-h-full flex-col">
      <PageHeader title={workspace.name} />
      <div className="mt-auto flex justify-center px-4 pb-6 pt-4">
        <SearchDock />
      </div>
    </div>
  );
}
