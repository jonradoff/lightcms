# Next steps — LightCMS / metavert.io

Handoff written 2026-10-09. Repo `~/lightcms` (GitHub `jonradoff/lightcms`), production site metavert.io on Fly app `metavert-cms`.

## Just wrapped

- **7.4.3 is merged, tagged and deployed** (2026-10-09): `main` at `368e0f3`, tag `v7.4.3`, live on metavert.io. Verified: `POST /cm/upload` answers 405 (was 403), public pages 200, one machine (`d890122a371528`).
- **The 166 leftover fork copies are purged** (6 + 69 + 14 + 77 across the four merged forks). `repair_fork_damage` dry-run afterwards: nothing to repair, 2,627 pages checked.
- **7.4.4 is written** (PR #8, branch `fixes-7.4.4`): OAuth `redirect_uri` is required; service `DeleteContent` clears `published` and `RestoreContent` publishes again (`published_before_delete`); the Search tool sample script builds elements; `fork_id: nil` everywhere. One clean full `go test -p 1 ./...` run. The sample script was not checked in a browser. Jon approved merging and deploying it on 2026-10-09 — check `git log main` and the admin sidebar version to see whether that finished.
- Content work on metavert.io is done and live: 166 pages (121 new, 45 updated). Working files are in `~/metavert-geo-expansion/`.

## Exactly where we are

- **Not verified:** whether metavert.io has active webhooks. Since 7.4.3 any that exist receive events they were silently missing.
- `server.json` still pins 7.3.4 release assets and hashes.

## How we continue

1. **LLM Optimizer product update** (`~/llmopt`), deferred: update `research.md`, the Research Citations page in `frontend/src/App.tsx`, and the scoring prompts/weights in `backend/main.go` from the research refresh. Present as a numbered plan for approval first.
2. Deploys: `./deploy.sh` from `~/lightcms` only, with `export FLY_ACCESS_TOKEN="$(awk '/^access_token:/{print $2}' ~/.fly/config.yml)"`. About 8 minutes. Then `go build -o bin/lightcms-mcp ./cmd/mcp`.

### Blocked on Jon

- **Public release** (GitHub release, MCPB bundles for darwin/linux arm64/amd64, `server.json` hashes, `mcp-publisher publish`): the permission classifier blocks it. Jon runs it or grants permission. Go straight to the latest tag.
- **Stray Fly volume:** `fly volumes destroy vol_r7y0zqk6p92ll6nr -a metavert-cms` (unattached; blocked for the agent).
- **Webhooks:** look at `/cm/webhooks` on metavert.io and disable any that should not be firing.
- **Disk space on the Mac:** the 1.8 TB drive is essentially full (about 14 GB free). It broke a test run on 2026-10-07. `go clean -cache` frees about 9 GB temporarily; the real consumer has not been found.
- Unreviewed by Jon: the Beamable comparison pages; the `/agentic-web` infographic still shows retracted numbers (Jon said the page is OK regarding proprietary data).

## Do NOT redo

- **Do not fan out parallel agents against the metavert.io CMS.** Thirteen at once OOM-killed the machine on 2026-10-06. Writers produce local JSON; one loader pushes sequentially; check `get_agent_sandbox` before every write.
- **Do not deploy with `fly deploy`.** It creates an orphan machine; the live machine `d890122a371528` is a legacy one that only `./deploy.sh` updates.
- **Do not try to delete the four merged forks** — LightCMS refuses to delete a merged fork. Their copies are already purged.
- **Do not "publish all drafts"** on metavert.io: that would publish the leftover fork copies' neighbours and held pages. Publish by explicit ID list.
- **Do not restore content locking in the admin editor or `/cm/upload`** as a bug fix: the editor never took locks, and the upload route was dead since 1.x. Both were removed deliberately in 7.4.3.
- **Do not add Web3 angles to Beamable content**, and do not include Beamable's internal analysis data anywhere on the site.
- **Do not chase the flaky test failures** when they stall for 15–100 seconds with timeout text: that is the Atlas test cluster. See the Testing notes in `CLAUDE.md`.
