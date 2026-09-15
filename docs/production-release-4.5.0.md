# Production Release Candidate 4.5.0

Date: 2026-09-15

## Source and target

- Production baseline: GitLab `main` at `bf41906` (`3.4.12`).
- Accepted test source: GitLab `codex/containerize-single-image` at `8cb5d16` (`4.5.0`).
- Local-only candidate branch: `codex/prod-release-4.5.0`.
- No GitLab production branch, tag, build or deployment has been created.

## Included runtime scope

- Digital twin route at `/erzhuang-project/digitaltwin/`.
- System settings whitelist, defaulting to institution `10001` when no row exists.
- Existing user institution permissions intersected with the digital twin whitelist.
- Existing NVR camera metadata, authorization, live player, screenshot watermark and audit reuse.
- Approved camera mapping and treatment-room ordering.
- Three `SdyService` Triple RPC calls through ZooKeeper interface discovery.
- Immediate KPI request, 30-second visible-page polling, background pause and stale-value preservation.
- Existing seven-second demo BI chart rotation; no real BI trend integration in this release.
- Audit label and event details for digital twin whitelist changes.

## Production protections retained

- Production SSO logout and return-page behavior remain based on the `3.4.12` production source.
- Cookie cleanup retains the explicit `soyoung.com` parent domain for `lite.soyoung.com`.
- Test-only generic parent-domain cookie logic is not imported.
- `Dockerfile`, `nginx.conf` and `.gitlab-ci.yml` are unchanged from production.
- No force push, remote candidate branch, database write or instance configuration change is part of candidate preparation.

## Database and runtime dependencies

- No new table, field or index is required.
- Digital twin settings reuse the existing `tb_app_settings` table.
- A missing whitelist row safely defaults to `10001`; an explicit empty list disables all institutions.
- Camera viewing reuses the existing production NVR authorization configuration.
- RPC registry selection reads `APP_RUN_ENV` first and `GO_APP_RUN_ENV` second. Production must resolve to `prod`; the mapped registries are the five company `register-center*.sy.soyoung.com:2181` endpoints.
- No TLS, mTLS or new application Secret is introduced.

## Verification gates

- Frontend full unit suite and production build.
- Go digital twin, application and NVR test compilation.
- Linux `amd64`, `CGO_ENABLED=0` full build.
- Production SSO logout regression, including `soyoung.com` parent-cookie cleanup.
- Isolated Chrome desktop/mobile digital twin checks.
- Final tree comparison against the accepted test source, allowing only documented production-specific differences.

## Verification results

- Frontend Vitest: `19` files and `127` tests passed.
- Frontend production build completed successfully. Vite reported the existing large-chunk advisory only; it did not fail the build.
- Go full repository test suite passed with `CGO_ENABLED=0` after allowing the two existing `httptest` packages to bind local ephemeral ports.
- Production SSO logout regression passed and asserted both the host-only cookie and the `soyoung.com` parent-domain cookie are cleared.
- Linux `amd64`, `CGO_ENABLED=0` full repository build completed successfully.
- Isolated headless Chrome digital twin checks passed in desktop, laptop and mobile layouts: `7/7` scenarios.
- Fullscreen camera layout checks passed at `2560x1440`, `3840x2160`, `3440x1440` and `1440x900`; the player surface filled the viewport in all four cases.
- `git diff --check` passed.
- Runtime tree comparison against accepted test commit `8cb5d16` found only the intentional production SSO implementation/test difference and a formatting-only `gofmt` change in the digital twin NVR route registration.

## Rollback

- Before production push, the stable rollback source remains `bf41906` (`3.4.12`).
- The production candidate will be delivered as a small release commit on top of `bf41906`, so reverting that release commit restores the previous production source without rewriting history.
