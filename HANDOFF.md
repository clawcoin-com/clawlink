# ClawLink — Session Handoff

> Paste or reference this file at the start of the next session so Sisyphus can
> pick up without re-reading all the chat history.
>
> **Authoritative rule set lives in `docs/final-rules.md`. Read that first.**

---

## Repo layout

- Main repo: `D:\work\clawcoin-com\clawlink` (Go API + Nuxt frontend + docs + deploy)
- Sibling CLI repo: `D:\work\clawcoin-com\clcli` (official CLI client)
- Both repos are on branch `develop`. Neither has been pushed to remote yet.
- Both must stay aligned with `docs/final-rules.md`.

---

## Current versions

- clawlink: `v0.0.31` (latest tag on develop)
- clcli:    `v0.0.17` (latest tag on develop)

---

## Current state (as of this handoff)

### Infrastructure / auth (carried over, still valid)
- Auth model finalized: `register-agent` accepts only `username+password` OR
  `wallet+signature`; `email` field is rejected.
- Login returns JWT via one-time code exchange (`/auth/exchange`).
- Public skill docs exposed at `/skill.md` and `/api/v1/skill/docs`.
- Git author rewrite script at `scripts/rewrite-git-author.ps1`.
- Correct git commit author name: `osiclaw` (`osindex@clawcoin.com`).
- Release pipeline: `release.yml` builds multi-platform binaries;
  `publish-npm.yml` triggers on `workflow_run: [release] completed` with tag `v*`.
- Deprecated direct-reply endpoint fully removed; queue-only flow enforced.

### Comments / replies — fixed and shipped this session

#### reply_count fixes
- `GET /feed` and `GET /feed/following` now call `attachCounts` so homepage
  cards show real `reply_count` instead of 0. (v0.0.24)
- `GET /posts/:id` was returning `reply_count=0` because `attachCounts` was
  called on a copy-of-value. Fixed to use `single := []models.Post{post}`;
  detail page and homepage now show the same count. (v0.0.25)
- `PostHandler.attachCounts` method now delegates to package-level
  `attachCounts(posts, db)` shared with `FeedHandler`. (v0.0.24)

#### Nested reply pagination (v0.0.26 / v0.0.27)
- `GET /posts/:id/replies` supports `?parent_id=&limit=&cursor=` — paginates
  by direct children only (not full subtree).
- Each reply carries `child_count` (direct child total) plus up to 3 preview
  children loaded inline by the backend (`attachReplyChildPreviews`).
- Frontend `usePost.ts` uses `api.getList` for both root and child pages.
- `ReplyTree.vue` shows `Show N reply/replies` or `Show more` depending on
  whether any children have already been loaded.
- Orphan nested replies (parent deleted) are returned as root nodes.

#### Duplicate / spam agent reply prevention (v0.0.28)
- `rejectIfTooSimilar` now covers nested replies (previously top-level only),
  comparing against recent replies in the same parent scope.
- Same agent + same parent: 24-hour cooldown before a second reply is allowed.
- Per-parent agent cap (`subthreadAgentReplyCap = 8`): at most 8 distinct
  direct agent replies under any single comment.
- `heartbeat enrichSubthreadContextForAgent` no longer recommends parents that
  the requesting agent has already replied to, or parents already at agent cap.
- Trigger loop root cause: `needs_rating` drops a post once it has >= 4
  ratings; `feed_interesting` sorts by `score DESC` so 0-reply posts score 0.
  Both of these left many new posts with 0 agent replies.

#### needs_reply trigger — new (v0.0.31)
- Added `needsReplyTrigger` in `api/internal/skill/triggers.go`.
- Selects posts that are: in an agent-allowed board, <= 7 days old, have
  >= 4 ratings (gate cleared), have ZERO agent replies, and are not flooded.
- Priority: medium, between `needs_rating` and `silent_too_long`.
- Payload: `{ type: "needs_reply", priority: "medium", post_ids, rating_counts }`.
- Updated skill.md trigger table with new entry.

#### UI depth / display fixes (v0.0.29 / v0.0.30)
- Deep nested replies: after depth 3, indentation stops entirely; depth 4
  renders one dashed border; depth 5+ renders no border and no indent — only
  a `· depth N compact thread view` dot label. This prevents dashed lines from
  stacking into a solid stripe.
- Root-level comment collapse (TOP_LEVEL_VISIBLE=12) removed; server pagination
  replaces it.
- `Show 1 replies` → `Show 1 reply` / `Show N replies` / `Show more` correctly
  pluralised. (v0.0.31)
- `Showing X of Y comments` status line added below reply list.

### Security fixes (v0.0.27)
- `User.Email`, `User.EmailVerified`, `User.Metadata` all set to `json:"-"` in
  the model — never leak in any API response including Post/Reply Author embeds.
- `PublicUser` struct also has `Email` removed.
- Public profile page `u/[wallet].vue` no longer renders `user.email`.

### submolt post list pagination (latest commit b70c417)
- `frontend/pages/s/[id].vue` now loads posts with `Load More` button;
  previously loaded all posts at once.

---

## clcli changes this session

### Weighted random trigger selection (v0.0.17)
- New file: `internal/daemon/trigger_pick.go` — `pickTriggerWeighted()`.
- Replaces the old `trig := hb.Triggers[0]` pattern.
- Weights: high 0.70 / medium 0.25 / low 0.05.
- `review_due` short-circuits unconditionally (15-min deadline, never miss it).
- Empty buckets auto-collapse and remaining buckets renormalize.
- Within a chosen bucket, uniform random selection.
- `internal/daemon/daemon.go` updated to call `pickTriggerWeighted`.
- `package.json` npm version bumped to `0.0.17`.

---

## Architecture notes you must know

### Comments counting
- `Post.ReplyCount` is `gorm:"-"` — never persisted, always computed live.
- All list/feed/detail endpoints call `attachCounts` before serializing.
- Deleting replies automatically reduces visible count on next fetch.

### Agent reply trigger lifecycle
```
New post created
→ agents rate it (needs_rating trigger)
→ ratings >= 4 (rating gate clears)
→ needs_reply trigger surfaces it (NEW — bridges the gap)
→ agent takes queue slot + submits reply
→ more agents see it via reply_to_me / discussion_reply / feed_interesting
```

### Trigger priority in clcli
Server sends: `review_due` → `mention/reply_to_me/discussion_reply` → `needs_rating`
→ `needs_reply` → `silent_too_long` → `feed_interesting`.
clcli picks ONE per cycle using weighted random (not first-always).

### Nested reply depth
- No server-side depth limit enforced.
- Frontend compresses visually at depth > 3.
- SQL for `needs_reply` and `subthreadRoots` filters flooded/saturated threads.

### Board agent scope
- Agent posts/replies are only allowed in boards whose name contains `"agent"`
  (case-insensitive). Checked in `submoltAllowsAgents()`.

---

## Known issues / deferred work

- Orphan replies (parent deleted but children remain): fallback renders them as
  root nodes. No cleanup SQL has been run yet. Suggested cleanup:
  ```sql
  -- Step 1: preview
  WITH RECURSIVE reply_tree AS (
    SELECT id, post_id, parent_id, 0 AS depth ...
  ) SELECT * FROM reply_tree WHERE depth > 3;
  ```
- Historical duplicate agent replies: ~154 kai-linden replies under one parent
  detected live. Cleanup SQL provided in session but not yet executed:
  ```sql
  -- Preview per-agent duplicate nested replies
  SELECT post_id, parent_id, author_id, COUNT(*) AS reply_count ...
  FROM replies r JOIN users u ON u.id = r.author_id
  WHERE r.parent_id IS NOT NULL AND u.is_agent = TRUE
  GROUP BY ... HAVING COUNT(*) > 1;
  -- Delete keeping latest 1
  DELETE FROM replies WHERE id IN (WITH ranked AS (...) SELECT id FROM ranked WHERE rn > 1);
  ```
- Historical duplicate top-level agent replies: `tess.w` has 6 near-identical
  top-level replies in one post. Same cleanup pattern, partitioned by
  `(post_id, author_id)` where `parent_id IS NULL`.
- Neither `go test ./...` coverage tests nor new unit tests were written for the
  new trigger functions. Existing test suite still passes.
- `clcli build/clcli.exe` binary in the repo may be stale. Rebuild with:
  ```powershell
  cd D:\work\clawcoin-com\clcli; make build-windows
  ```
- Docker API container has not been rebuilt; live server still runs the previous
  image. Rebuild with:
  ```powershell
  cd D:\work\clawcoin-com\clawlink; docker-compose build --no-cache api && docker-compose up -d --force-recreate api
  ```

---

## Known traps for next session

- **Docker build cache serves stale skillDoc.** Always `--no-cache` on API rebuild.
- **`clcli build/clcli.exe` may be stale.** Rebuild before testing CLI behavior.
- **Nuxt runtimeConfig env names.** Only `NUXT_API_BASE` and
  `NUXT_PUBLIC_API_BASE` are valid.
- **`register-agent` must reject email.** Regression if it ever accepts `{email, password}`.
- **`needs_reply` uses `sub_molts` table name** (GORM default snake_case plural
  for `SubMolt`). If the table name changes, update the SQL join.
- **`rejectIfTooSimilar` parentID pointer:** the function signature changed from
  `(postID, agentID, content)` to `(postID, agentID, *string, content)`.
  Any new callers must pass the parent pointer, not omit it.
- **clcli weighted trigger does not persist state between cycles.** If a
  high-priority trigger fires every cycle, medium/low still get picked ~30% by
  probability — no deficit carry-over. This is intentional.

---

## Agent hygiene for the next session

- Read short slices with `Read_tool` using `offset` + `limit`; avoid full-file reads.
- `Grep_tool` with `head_limit` and narrow patterns; avoid `output_mode=content`
  on broad queries.
- When delegating with `Task`, demand a PASS/FAIL + exact file paths only,
  not pasted source.
- When asked "what did we do so far", refer to this file and
  `docs/final-rules.md` instead of re-listing history.
- Prefer doing one unit of work, confirming, then the next.

---

## How to continue in a fresh session

Open the new chat and say:

> Read `docs/final-rules.md` and `HANDOFF.md`. Then confirm current state,
> and tell me what immediate next steps remain.
