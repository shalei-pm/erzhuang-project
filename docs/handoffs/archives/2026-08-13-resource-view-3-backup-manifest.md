# 2.x Source Backup Archive Manifest

日期：2026-08-13

## Archive

- Zip file: `erzhuang-project-2.31.8-before-resource-view-3.zip`
- Purpose: preserve a complete source snapshot of the 2.x stable codebase before the 3.0 resource-view iteration starts, together with handoff documents that explain how to read and roll back this snapshot.

## Included Files

- Complete source tree from Git tag `v2.31-stable-before-resource-view-3`.
- `docs/handoffs/2026-08-13-2x-stable-backup-before-resource-view-3.md`
- `docs/superpowers/specs/2026-08-13-store-space-resource-view-3-design.md`
- `docs/superpowers/plans/2026-08-13-store-space-resource-view-3-implementation.md`
- `docs/decisions.md`
- `work/current-plan.md`

## Excluded Files

- `.git/` metadata is not included; this is a portable source archive, not a Git repository clone.
- Runtime secrets, tokens, DSNs, database passwords, and local environment files are not included.
- Full historical state files are not added on top of the 2.x source snapshot to avoid carrying old environment examples into a portable handoff layer.

## Stable 2.x Reference

- Tag: `v2.31-stable-before-resource-view-3`
- Commit: `7311bda`
- Version: `2.31.8`

## How To Use

1. Unzip the archive.
2. Read the 2.x stable handoff first.
3. Read the 3.0 design.
4. Read the 3.0 implementation plan.
5. Use `docs/decisions.md` and `work/current-plan.md` for product decisions and current execution status.
6. If source-level history is needed, use the Git tag in the live repository instead of this zip:

```bash
git show --stat v2.31-stable-before-resource-view-3
git switch -c codex/inspect-2x-stable v2.31-stable-before-resource-view-3
```
