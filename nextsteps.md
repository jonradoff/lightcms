# Next steps — LightCMS / metavert.io

Handoff written 2026-10-09. Repo `~/lightcms` (GitHub `jonradoff/lightcms`), production site metavert.io on Fly app `metavert-cms`.

## Just wrapped

- **7.4.0, 7.4.1, 7.4.2 are merged, tagged and deployed.** `main` is at `4d22b05` (tag `v7.4.2`), live on metavert.io.
- **7.4.3 is written and waiting in PR #7** (https://github.com/jonradoff/lightcms/pull/7), branch `fixes-7.4.3`, commits `6a3a4ca` and `79882de`, plus the docs commit that adds this file. Not merged, not deployed.
  - Fixes the 12 problems left open after 7.4.2.
  - Three more found while verifying: pages soft-deleted through the API/MCP were still served; webhooks fired from a web request were dropped (delivery ran on the request context); fork preview pages were sent with public cache headers.
  - Follow-ups: RSS import through the SSRF guard, folder rename no longer touches sibling folders (`/blog` vs `/blogging`), confirmation on Disable Account / Reset Password, audit entries for webhook and import API calls.
  - Detail: `CHANGELOG.md` under 7.4.3.
- **Content work on metavert.io is done and live:** 166 pages (121 new, 45 updated) from the GEO expansion and the AI reading list. Working files are in `~/metavert-geo-expansion/`.
- Issue #6 filed: backlog idea for a role between editor and admin for site design.

## Exactly where we are

- Working tree is clean on `fixes-7.4.3`, pushed. PR #7 is open and mergeable.
- **Verified:** Playwright 151/151 on the final build (local server, `lightcms-test` database). Script: session scratchpad `uicheck6/pw/check6.js` + `extra6.js` — the scratchpad is wiped on session restart, so treat the scripts as gone; screenshots survive in `~/metavert-geo-expansion/ui-check-6/`.
- **Verified with a caveat:** every Go test passes on the final tree, but not in one uninterrupted run. The Atlas test cluster kept timing out; four full runs each failed a different handful of tests on timeouts, and each failed test passed when re-run. One clean full run was obtained before the follow-up commit.
- **Not verified:** whether metavert.io has active webhooks (the read was blocked). After 7.4.3 deploys, any that exist will start receiving events they were silently missing.
- `bin/lightcms-mcp` was last rebuilt at 7.4.2. 7.4.3 adds no MCP tools.
- `server.json` still pins 7.3.4 release assets and hashes.
- 166 leftover fork copies (drafts) still sit in production from the four merged forks: `6ac54b4bb1ffd9b2689435ce`, `6ac554e29dee2061a1d173b2`, `6ac5784e9dee2061a1d1748d`, `6ac596189dee2061a1d174d4`. They are hidden from listings and harmless, but should be purged.

## How we continue

1. **Merge and deploy 7.4.3** — only when Jon says so explicitly ("merge and deploy").
   - `gh pr merge 7 --merge`, `git checkout main && git pull`, `git tag v7.4.3 && git push origin v7.4.3`.
   - Deploy with `./deploy.sh` from `~/lightcms`. **Never plain `fly deploy`.** Fly auth in a non-login shell: `export FLY_ACCESS_TOKEN="$(awk '/^access_token:/{print $2}' ~/.fly/config.yml)"`. It takes about 8 minutes and looks hung; it is not.
   - Verify (there is no version endpoint): `curl -s -o /dev/null -w '%{http_code}' -X POST https://metavert.io/cm/upload` should be 404 or 405 (the route is removed in 7.4.3); a few public pages return 200; the admin sidebar shows v7.4.3.
   - Rebuild the MCP binary: `go build -o bin/lightcms-mcp ./cmd/mcp`.
2. **Purge the leftover fork copies** (needs a session started after the MCP binary was rebuilt, so `purge_fork_copies` and `repair_fork_damage` are loaded). For each of the four fork IDs above: `purge_fork_copies` with `dry_run: true`, show Jon the list, then purge on his word. Then `repair_fork_damage` with `dry_run: true` and show the result. One caller, sequential — never parallel agents against this CMS.
3. **Remaining small fixes** (a 7.4.4 branch): OAuth `ValidateClient` skips the redirect check when `redirect_uri` is empty; service `DeleteContent` leaves `published: true` on soft-deleted rows; the Search tool page's sample script builds `innerHTML` from unescaped titles; older queries still use `fork_id: {$exists: false}` instead of `nil`.
4. **LLM Optimizer product update** (`~/llmopt`), deferred since the start of the session: update `research.md`, the Research Citations page in `frontend/src/App.tsx`, and the scoring prompts/weights in `backend/main.go` from the research refresh. Present as a numbered plan for approval first.

### Blocked on Jon

- **Public release** (GitHub release, MCPB bundles for darwin/linux arm64/amd64, `server.json` hashes, `mcp-publisher publish`): the permission classifier blocks it. Jon runs it or grants permission. Go straight to 7.4.3 once it is merged.
- **Stray Fly volume:** `fly volumes destroy vol_r7y0zqk6p92ll6nr -a metavert-cms` (unattached; blocked for the agent).
- **Webhooks:** look at `/cm/webhooks` on metavert.io before deploying 7.4.3 and disable any that should not start firing.
- **Disk space on the Mac:** the 1.8 TB drive is essentially full (about 14 GB free). It broke a test run on 2026-10-07. `go clean -cache` frees about 9 GB temporarily; the real consumer has not been found.
- Unreviewed by Jon: the Beamable comparison pages; the `/agentic-web` infographic still shows retracted numbers (Jon said the page is OK regarding proprietary data).

## Do NOT redo

- **Do not fan out parallel agents against the metavert.io CMS.** Thirteen at once OOM-killed the machine on 2026-10-06. Writers produce local JSON; one loader pushes sequentially; check `get_agent_sandbox` before every write.
- **Do not deploy with `fly deploy`.** It creates an orphan machine; the live machine `d890122a371528` is a legacy one that only `./deploy.sh` updates.
- **Do not try to delete the four merged forks** — LightCMS refuses to delete a merged fork. Purge their copies instead.
- **Do not "publish all drafts"** on metavert.io: that would publish the leftover fork copies' neighbours and held pages. Publish by explicit ID list.
- **Do not restore content locking in the admin editor or `/cm/upload`** as a bug fix: the editor never took locks, and the upload route was dead since 1.x. Both were removed deliberately in 7.4.3.
- **Do not add Web3 angles to Beamable content**, and do not include Beamable's internal analysis data anywhere on the site.
- **Do not chase the flaky test failures** when they stall for 15–100 seconds with timeout text: that is the Atlas test cluster. See the Testing notes in `CLAUDE.md`.
