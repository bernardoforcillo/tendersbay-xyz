import { useCallback } from 'react';
import { useAsync } from '~/hooks';
import { espdClient } from '~/lib/api/client';

/**
 * A bid's export history — the fact of each export, never its bytes.
 *
 * Read by the export sheet so a person can tell "I already sent the 2.1.1 XML"
 * from "I meant to". The server stores a content hash rather than the file, so
 * this can say WHAT was exported and when, and deliberately cannot re-serve it:
 * a re-export re-composes from the current dossier, which is the honest answer
 * when the dossier has moved on.
 */
export function useEspdExports(workbenchId: string, bidId: string) {
  const fn = useCallback(
    () => espdClient.listExports({ workbenchId, bidId }).then((r) => r.exports),
    [workbenchId, bidId],
  );
  return useAsync(fn);
}
