# cn2-sw pinned release procedure

Deployment issue: https://github.com/jihtsan/sub2api/issues/3

The application target is `e1f7e4b8206067131c77611671af003618b4a4f9`.
Production initially runs version 0.2.5, commit
`86f93c28ee34cc74b629dafb748bd5ac5ca8c5ea`, inside a container created
from a 0.1.136 image. Recreating that container from its original image
would lose the in-container application upgrade.

## Scope

Replace only the application and embedded frontend. Preserve PostgreSQL,
Redis, mounted data, existing environment, authentication secrets, and account
configuration. Do not enable RPM, observation, fingerprint convergence, or
WS routing as part of this release. Do not submit the browser's draft form.

The target and production have 286 matching migration checksums. No new
migration is required. The target also includes intervening upstream fixes;
it is not solely the shared-account patch.

## Rehearsal evidence (2026-09-19)

- Frontend production build (including locale checks and TypeScript) and
  embedded Linux AMD64 Go build passed.
- All 286 production migration checksums match the target source.
- The full PostgreSQL backup restored successfully in the internal network.
- Both the new image and rollback image passed startup, clone-user login,
  authenticated account APIs, embedded UI, simulated streaming/non-streaming
  requests, and billing checks on the cloned database.
- Each two-request run added exactly two usage records, 20 input tokens,
  four output tokens, and positive billed cost.
- The new traffic-control API reports both optional policies disabled and
  Redis-backed state available. Actual external egress was tested and blocked.
- The existing Compose environment matches the live container environment.
- New binary SHA256:
  `82017d0e3c17b394d4f79768555f05049f6534af3aea22fa513b9d75cacd651b`.
- Original running binary SHA256:
  `2d9d876865c302a19046afe4693c729361bf5de21312d538f6fd3b3085120d2b`.

Server-local backup and rehearsal reports are retained in a root-only release
directory. Clone-only users, synthetic upstreams, and compliance fixtures do
not change any production user or represent acceptance of legal terms.

## Build and rollback artifacts

1. Save the actual running executable, inspect metadata, Compose file, `.env`,
   mounted application data, and a PostgreSQL custom-format dump in a root-only
   server directory. Never put these backups or credentials in Git.
2. Verify the running process executable hash matches the saved executable.
3. Build the frontend from the target commit and compile its embedded Go
   application with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`, recording the
   source commit, version, build time, and binary checksum.
4. Use `Dockerfile.binary-release` twice: once with the saved production binary
   for rollback, once with the new binary. Pin the known-working runtime base
   by digest. Retaining that runtime intentionally avoids an unrelated OS or
   backup-client upgrade; it is not a claim that the old base is fully patched.
5. Record immutable image IDs and export both images outside the Docker image
   store. Copy only the binary into the build context, never production config.

## Rehearsal

Restore the database dump into an independent PostgreSQL instance. Use an
independent Redis instance, copied application data, and an internal Docker
network with no production network membership or public ports. Explicitly
override database/Redis destinations and disable token refresh and optional
maintenance jobs. Block outbound access even if a background job lacks a
disable switch. Verify the isolation before starting an application.

Check startup/migration validation, embedded UI, health, authentication,
account APIs, and controlled synthetic request paths. Any test fixture or
credential change must be confined to the clone. Rehearse rollback on the
same cloned database using the saved production binary. Remove rehearsal
containers and their temporary secrets once verification is complete.

## Cutover

1. Check current load and in-flight requests. Temporarily stop admitting new
   public connections and allow existing requests to drain; persistent sockets
   may need to reconnect. Take the final backup before switching.
2. Retain the existing production Compose and `.env`. Add the pinned release
   overlay and set `SUB2API_RELEASE_IMAGE` to the verified immutable image ID.
3. Validate the resolved Compose without printing its secrets. Recreate only
   `sub2api` using `up -d --no-deps --pull never sub2api`. Never use `down -v`.
4. Check health, actual process version/hash, UI assets, authentication,
   migration status, a bounded real request when an eligible account exists,
   and usage accounting. Restore public admission promptly.
5. If startup, authentication, accounting, or requests regress, select the
   verified rollback image ID using the same overlay and recreate only the
   application. Verify recovery. Do not restore an old database over new
   production transactions as a routine application rollback.

## Ongoing operation

Use both Compose files for future application changes. A plain command using
only the old base Compose can revert the image selection. Do not use the
upstream application's online update action: it still downloads from
`Wei-Shaw/sub2api` and can overwrite the custom build. Keep account policies
unchanged for the initial observation period. Evaluate concurrency, WS, and
fingerprint changes separately after the application release is stable.
