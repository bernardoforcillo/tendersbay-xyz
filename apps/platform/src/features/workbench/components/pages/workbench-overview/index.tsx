import { SearchDock } from '~/features/account/components/organisms';

export function WorkbenchOverviewPage() {
  // Mirror the workspace overview: the search dock pins to the bottom and
  // animates to center on Explore via its shared layoutId.
  return (
    <div className="flex flex-1 flex-col">
      <div className="mt-auto flex justify-center px-4 pb-6 pt-4">
        <SearchDock />
      </div>
    </div>
  );
}
