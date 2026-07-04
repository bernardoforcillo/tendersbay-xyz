import { SearchDock } from '~/features/account/components/organisms';

export function WorkspaceOverviewPage() {
  return (
    // Dock pinned to the bottom — animates to center on Explore via its shared layoutId.
    <div className="flex min-h-full flex-col">
      <div className="mt-auto flex justify-center px-4 pb-6 pt-4">
        <SearchDock />
      </div>
    </div>
  );
}
