# Personal data (PII)

Applies to `services/backend` and `apps/platform`. The product stores two kinds of personal
data, and the second has stricter rules than the first.

## Account holders

`users` (email, display name, credentials — owned by authlayer) and everything keyed by a
`user_id` (`stated_by`, `created_by`, `imported_by`, `exported_by`). Deletion is
`core/user.DeleteAccount`, which authlayer performs; audit columns that reference a user
are plain uuids with no foreign key, so a deleted account leaves its provenance trail
intact but unresolvable — that is deliberate: a Part III declaration keeps *that someone*
stated it even after the account is gone.

## Third parties named in the dossier — `company_representatives`

The one table holding personal data of people who are **not** the account holder: the
operator's legal representatives and procurators (DGUE Part II.B — name, birth date and
place, address, email). Rules:

- **Purpose-bound.** Read only to compose an ESPD/DGUE (`core/espd`) and to render the
  dossier to workspace members. Never joined into analytics, never a PostHog property,
  never in a log line — `SourceNote` is the only free text that may reach a log, and a
  representative has none.
- **Lifecycle = the workspace.** `company_representatives.workspace_id` references
  `workspaces ON DELETE CASCADE`; deleting the workspace deletes them, and there is no
  other copy. Do not add a soft-delete or an archive table.
- **Export = the document.** The only place a representative leaves the backend is the
  generated ESPD/DGUE, which the workspace requests deliberately; the export audit
  (`bid_espd_exports`) stores a content hash, never the bytes.
- **Retention.** Same as the dossier: until the operator removes the row or the workspace.
  A representative who leaves the company is removed by the workspace, not aged out.
- **Adding a column** to this table, or a second table of third-party personal data, is a
  policy change — update this rule in the same PR.

The per-bid party tables (`bid_subcontractors`, `bid_reliances`) hold *company* names and
VAT numbers, not natural persons, and are outside this rule.
