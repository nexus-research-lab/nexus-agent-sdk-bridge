# Preserved sandbox working draft

Historical, non-normative material saved while consolidating sandbox development
on `codex/desktop-sandbox-approvals` on 2026-09-27. This archive is not compiled
and does not add any public Bridge API.

`working-tree.patch.gz` preserves the two untracked `settings_receipt_store` source
and test files left in the older checkout at
`eeaff7df69f324bf5d1e346692e30f39235ce0c5`. The original checkout and its working
files are retained at that same commit with a detached HEAD. `manifest.json`
records file and patch hashes; replay against the original base reproduced
both files byte for byte.

This was an unfinished receipt-store experiment, with no production callers.
In particular, its `Begin` method does not block new writes behind pending or
unknown receipts, despite the included test expecting that protection. It also
does not establish cross-process coordination or durable reconciliation. Do not
copy the draft into the active `client` package or treat it as implemented
functionality. The maintained Bridge boundary remains the runtime contract.

For historical recovery, decompress and apply the patch only in a separate checkout of the
recorded base. Continue product development on `codex/desktop-sandbox-approvals`.
