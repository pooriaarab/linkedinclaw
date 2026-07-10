# linkedinclaw crawlkit mirror Implementation Plan

> **Execution model (overrides default):** Do NOT dispatch Claude subagents for
> implementation. Execution = `gemini-personal` CLI. Task decomposition + review +
> iteration loop = `glm5.2` via `pi -p`. Final diff review before merge = `codex-personal`
> (`CODEX_HOME=~/.codex-personal`). Claude's role here is: hand this plan to glm5.2,
> read back its report, sanity-check against spec, decide go/no-go, do the actual
> `git merge`/push itself. Full rationale: `docs/superpowers/specs/2026-07-09-linkedinclaw-crawlkit-mirror-design.md`.

**Goal:** Turn empty `pooriaarab/linkedinclaw` into a crawlkit-based Go CLI that mirrors
a user's own LinkedIn account (profile, connections, messages, own posts, saved posts,
followed companies) into local SQLite, with a companion MV3 extension for passive
`li_at` cookie refresh.

**Architecture:** Go binary on `github.com/openclaw/crawlkit` (config/store/state/
mirror/tui/output/control packages reused as-is). linkedinclaw owns: Voyager API
client, GDPR export parser, auth/cookie handling, schema, CLI commands. A separate
`extension/` MV3 extension passively captures the LinkedIn session cookie and POSTs it
to a localhost listener the CLI exposes — no native-messaging-host install needed.

**Tech Stack:** Go 1.26+, crawlkit, SQLite (mattn/go-sqlite3 or modernc, whichever
crawlkit's store package already assumes), agent-browser skill (bootstrap login /
export-request automation), TypeScript MV3 extension (Manifest V3, `chrome.cookies`
API).

---

## File Structure

```
linkedinclaw/
  go.mod
  cmd/linkedinclaw/main.go
  internal/
    auth/
      session.go          # keyring read/write, env resolution, session.json promotion
      listener.go          # localhost HTTP listener for extension cookie POSTs
    voyager/
      client.go            # authenticated HTTP client, rate limiter, backoff
      profile.go
      connections.go
      conversations.go     # conversations + messages
      posts.go              # own authored posts
      saved.go              # saved/bookmarked posts
      companies.go          # followed companies
      client_test.go        # fixture-based tests
    exportzip/
      parse.go              # GDPR zip -> typed rows
      parse_test.go
    schema/
      schema.go             # DDL, migrations via crawlkit/store
      schema_test.go
    cli/
      root.go
      init.go
      doctor.go
      login.go               # shells agent-browser
      sync.go                # --source api|export|both, --full
      export.go              # request (shells agent-browser) + import
      search.go
      messages.go
      tui.go                 # wraps crawlkit/tui
      status.go               # status/metadata/diagnostics --json
      checkupdate.go
      mirror.go               # subscribe/publish via crawlkit/mirror
  extension/
    manifest.json
    background.ts
    README.md                 # install steps, what it does, what it never does
  docs/superpowers/specs/2026-07-09-linkedinclaw-crawlkit-mirror-design.md  (done)
  docs/superpowers/plans/2026-07-09-linkedinclaw-crawlkit-mirror.md          (this file)
  README.md                    # rewritten: scope, disclaimer, install, quick start
```

---

## Task 1: Repo scaffold + crawlkit dependency

**Files:**
- Create: `go.mod`, `cmd/linkedinclaw/main.go`
- Modify: `README.md`

- [ ] **Step 1:** `go mod init github.com/pooriaarab/linkedinclaw && go get github.com/openclaw/crawlkit@latest`
- [ ] **Step 2:** `cmd/linkedinclaw/main.go` — minimal cobra (or crawlkit's existing CLI helper, check what discrawl/slacrawl use for their root command and match it) root command that prints version and exits 0.
- [ ] **Step 3:** Run: `go build ./... && ./linkedinclaw --version` — expect it to print a version string, no error.
- [ ] **Step 4:** Rewrite `README.md` top section with the scope + disclaimer paragraph from the design spec's "Why" and "Non-goals" sections verbatim.
- [ ] **Step 5:** Commit: `git add go.mod go.sum cmd README.md && git commit -m "Scaffold linkedinclaw on crawlkit"`

## Task 2: Schema + store wiring

**Files:**
- Create: `internal/schema/schema.go`, `internal/schema/schema_test.go`

- [ ] **Step 1:** Write failing test in `schema_test.go`: open a temp SQLite DB via `crawlkit/store`, call `schema.Migrate(db)`, assert tables `profile`, `connections`, `conversations`, `messages`, `posts`, `saved_posts`, `companies_followed` all exist (`SELECT name FROM sqlite_master WHERE type='table'`).
- [ ] **Step 2:** Run test — expect FAIL (`schema.Migrate` undefined).
- [ ] **Step 3:** Implement `schema.Migrate` with DDL:
  ```sql
  CREATE TABLE profile (id INTEGER PRIMARY KEY CHECK (id=1), urn TEXT, first_name TEXT, last_name TEXT, headline TEXT, updated_at TEXT);
  CREATE TABLE connections (urn TEXT PRIMARY KEY, first_name TEXT, last_name TEXT, headline TEXT, company TEXT, connected_at TEXT, source TEXT); -- source: 'api' | 'export'
  CREATE TABLE conversations (urn TEXT PRIMARY KEY, participants TEXT, last_activity_at TEXT);
  CREATE TABLE messages (urn TEXT PRIMARY KEY, conversation_urn TEXT REFERENCES conversations(urn), sender_urn TEXT, body TEXT, sent_at TEXT, source TEXT);
  CREATE TABLE posts (urn TEXT PRIMARY KEY, body TEXT, posted_at TEXT, like_count INTEGER, comment_count INTEGER);
  CREATE TABLE saved_posts (urn TEXT PRIMARY KEY, author TEXT, body TEXT, saved_at TEXT);
  CREATE TABLE companies_followed (urn TEXT PRIMARY KEY, name TEXT, followed_at TEXT);
  CREATE VIRTUAL TABLE messages_fts USING fts5(body, content='messages', content_rowid='rowid');
  CREATE VIRTUAL TABLE posts_fts USING fts5(body, content='posts', content_rowid='rowid');
  CREATE VIRTUAL TABLE saved_posts_fts USING fts5(body, content='saved_posts', content_rowid='rowid');
  ```
  Use `crawlkit/state` for the `sync_state` table rather than hand-rolling it — check `crawlkit/state`'s adapter API and wire `connections`, `conversations`, `posts`, `saved_posts`, `companies_followed` each as their own tracked category.
- [ ] **Step 4:** Run test — expect PASS.
- [ ] **Step 5:** Commit: `git add internal/schema && git commit -m "Add linkedinclaw schema + FTS5 indexes"`

## Task 3: Auth — session storage + localhost listener

**Files:**
- Create: `internal/auth/session.go`, `internal/auth/session_test.go`, `internal/auth/listener.go`

- [ ] **Step 1:** Write failing test: `session.Resolve()` returns `(liAt, jsessionId string, err error)`; given `LINKEDINCLAW_LI_AT`/`LINKEDINCLAW_JSESSIONID` env vars set, returns those without touching keyring.
- [ ] **Step 2:** Implement `session.Resolve()`: env vars first, then OS keyring item `linkedinclaw`/`li_at` and `linkedinclaw`/`jsessionid` (shell out to `security`/`secret-tool` matching discrawl's exact resolution pattern — read discrawl's token resolution code for the exact keyring service/account shape and mirror it 1:1, don't reinvent).
- [ ] **Step 3:** Implement `session.Store(liAt, jsessionId string) error` — writes both to keyring.
- [ ] **Step 4:** Implement `listener.go`: `func Listen(port int) error` — starts an HTTP server on `127.0.0.1:<port>` (default port, made configurable in config.toml), exposes `POST /session` accepting `{"li_at": "...", "jsessionid": "..."}` JSON body, on receipt calls `session.Store` and also writes `~/Library/Application Support/linkedinclaw/session.json` (mode 0600) as a fallback record. Reject non-loopback connections at the `net.Listen` level (bind to `127.0.0.1` only, never `0.0.0.0`).
- [ ] **Step 5:** Run tests — expect PASS.
- [ ] **Step 6:** Commit: `git add internal/auth && git commit -m "Add linkedinclaw session storage + localhost listener"`

## Task 4: Voyager API client

**Files:**
- Create: `internal/voyager/client.go`, `internal/voyager/{profile,connections,conversations,posts,saved,companies}.go`, `internal/voyager/client_test.go`, `internal/voyager/testdata/*.json`

- [ ] **Step 1:** Write failing test in `client_test.go`: `NewClient(liAt, jsessionId string)` returns a client whose requests carry `Cookie: li_at=...; JSESSIONID=...` and `Csrf-Token: <jsessionid-unquoted>` headers (Voyager requires the CSRF token derived from JSESSIONID) — assert against an `httptest.Server` that inspects incoming headers.
- [ ] **Step 2:** Implement `Client` with a configurable rate limiter (`golang.org/x/time/rate`, default conservative: e.g. 20 req/min) and exponential backoff on HTTP 429/999 (cap retries at 5, then return a typed `ErrDeferred` the sync layer catches to skip-and-continue rather than abort the whole run).
- [ ] **Step 3:** For each of `profile.go`, `connections.go`, `conversations.go`, `posts.go`, `saved.go`, `companies.go`: implement a `Fetch...(ctx) ([]Row, error)` function against the corresponding Voyager endpoint, with a recorded fixture JSON response in `testdata/` and a table test asserting the parsed struct fields. (Exact Voyager endpoint paths/response shapes: capture these by recording real authenticated requests once during manual testing against your own account — do not guess field names from memory, LinkedIn's internal API shifts field names across snapshots.)
- [ ] **Step 4:** Run tests — expect PASS against fixtures.
- [ ] **Step 5:** Commit: `git add internal/voyager && git commit -m "Add Voyager API client with rate limiting"`

## Task 5: Sync command

**Files:**
- Create: `internal/cli/sync.go`
- Modify: `internal/schema/schema.go` (if cursor wiring needs adjustment)

- [ ] **Step 1:** Write failing integration test: `sync.Run(ctx, db, client, SourceAPI, full=false)` calls each `voyager.Fetch...`, upserts rows via `crawlkit/state`-tracked cursors, and on a mocked client returning `ErrDeferred` for one category, still completes the other categories and reports the deferred one in its return summary (not an error).
- [ ] **Step 2:** Implement `sync.Run` per the spec's `--source api|export|both` and `--full` semantics — `--full` bypasses the incremental cursor and refetches full history per category; incremental uses the cursor's last-seen timestamp/id.
- [ ] **Step 3:** Wire `cli/sync.go` cobra command: flags `--source` (default `both`), `--full`. `both` runs API sync then checks the Downloads folder (path from config, default `~/Downloads`) for a `*.zip` matching LinkedIn's export naming pattern not yet recorded as imported, and if found prints "found unimported export at <path>, run `linkedinclaw export import <path>`".
- [ ] **Step 4:** Run tests — expect PASS.
- [ ] **Step 5:** Commit: `git add internal/cli/sync.go && git commit -m "Add sync command with source selection and deferred-category handling"`

## Task 6: GDPR export parser + import command

**Files:**
- Create: `internal/exportzip/parse.go`, `internal/exportzip/parse_test.go`, `internal/exportzip/testdata/sample-export.zip`, `internal/cli/export.go`

- [ ] **Step 1:** Write failing test: `exportzip.Parse(path string) (Result, error)` given a fixture zip with known `Connections.csv`/`messages.csv` contents returns the expected typed rows; given a zip missing an expected file, returns a typed `ErrIncompleteExport` (not a partial write).
- [ ] **Step 2:** Implement `Parse` — unzip to a temp dir, locate and parse each expected CSV (LinkedIn's actual export file names/headers: verify against a real export from your own account before finalizing column mapping, don't guess).
- [ ] **Step 3:** Implement `cli/export.go`: `export import <zip>` calls `Parse`, then upserts into the same tables `sync.Run` populates, tagging `source='export'` on rows the API doesn't also provide, and validates via `ErrIncompleteExport` before writing anything.
- [ ] **Step 4:** Implement `export request` subcommand: shells out to the `agent-browser` skill's CLI entrypoint to drive LinkedIn Settings → "Get a copy of your data", selects all categories, submits, then prints "LinkedIn will email a download link. Run `linkedinclaw export import <path-to-zip>` once you have it."
- [ ] **Step 5:** Run tests — expect PASS.
- [ ] **Step 6:** Commit: `git add internal/exportzip internal/cli/export.go && git commit -m "Add GDPR export parser and export request/import commands"`

## Task 7: login command

**Files:**
- Create: `internal/cli/login.go`

- [ ] **Step 1:** Implement `login` command: shells out to `agent-browser`, instructing it to open linkedin.com/login, wait for the user to complete authentication (poll for an authenticated-page indicator, no timeout shorter than a few minutes since 2FA takes time), then extract `li_at`/`JSESSIONID` cookies from the browser context.
- [ ] **Step 2:** On success, call `auth.Store` (Task 3) with the extracted values, then print confirmation. On failure/timeout, print "login not detected — run `linkedinclaw login` again once you've finished signing in."
- [ ] **Step 3:** Manual test: run `linkedinclaw login`, confirm it stores retrievable credentials via `linkedinclaw doctor` (Task 9).
- [ ] **Step 4:** Commit: `git add internal/cli/login.go && git commit -m "Add login command via agent-browser"`

## Task 8: search, messages, tui commands

**Files:**
- Create: `internal/cli/search.go`, `internal/cli/messages.go`, `internal/cli/tui.go`

- [ ] **Step 1:** Write failing test for `search.go`'s query function: given seeded `messages`/`posts`/`saved_posts` FTS rows, `search.Query(db, "term")` returns matches ranked by FTS5 rank across all three tables.
- [ ] **Step 2:** Implement `search.Query` and the `search "<query>"` cobra command (text + `--json` output via `crawlkit/output`).
- [ ] **Step 3:** Implement `messages --person <name> --hours <n>` — filters `messages` joined to `conversations`/`connections` by participant name and a time window.
- [ ] **Step 4:** Implement `tui` command wrapping `crawlkit/tui`'s shared explorer, wiring linkedinclaw's tables into its entity/detail lane contract (match discrawl's `tui.go` wiring 1:1 as the reference).
- [ ] **Step 5:** Run tests — expect PASS.
- [ ] **Step 6:** Commit: `git add internal/cli/search.go internal/cli/messages.go internal/cli/tui.go && git commit -m "Add search, messages, and tui commands"`

## Task 9: doctor, status/metadata/diagnostics, check-update

**Files:**
- Create: `internal/cli/doctor.go`, `internal/cli/status.go`, `internal/cli/checkupdate.go`

- [ ] **Step 1:** Implement `doctor`: checks config loads, shows token resolution source (env/keyring/session.json), verifies a lightweight authenticated Voyager call succeeds, verifies DB+FTS wiring — mirror discrawl's `doctor` check list exactly, substituting LinkedIn-specific auth checks.
- [ ] **Step 2:** Implement `status --json`, `metadata --json`, `diagnostics --json` via `crawlkit/control`, matching the contract `crawlctl` expects from other downstream apps (check `crawlkit/control`'s doc comments for the exact struct shape and satisfy it, don't invent a new one).
- [ ] **Step 3:** Implement `check-update` via `crawlkit/releasecheck`.
- [ ] **Step 4:** Manual test: `linkedinclaw doctor` after a completed `login` — expect all checks green.
- [ ] **Step 5:** Commit: `git add internal/cli/doctor.go internal/cli/status.go internal/cli/checkupdate.go && git commit -m "Add doctor, status/metadata/diagnostics, and check-update commands"`

## Task 10: git-archive publish/subscribe

**Files:**
- Create: `internal/cli/mirror.go`

- [ ] **Step 1:** Implement `subscribe <git-url>` and `publish` wrapping `crawlkit/mirror`'s clone/init/pull/commit/push helpers, matching discrawl's `subscribe`/`publish` command wiring.
- [ ] **Step 2:** Manual test: `publish` against a scratch private repo, `subscribe` from a second local config pointed at that repo, confirm `search` works read-only there with no LinkedIn credentials configured.
- [ ] **Step 3:** Commit: `git add internal/cli/mirror.go && git commit -m "Add git-backed archive publish/subscribe"`

## Task 11: companion MV3 extension

**Files:**
- Create: `extension/manifest.json`, `extension/background.ts`, `extension/README.md`

- [ ] **Step 1:** `manifest.json`: MV3, `permissions: ["cookies"]`, `host_permissions: ["*://*.linkedin.com/*", "http://127.0.0.1/*"]`, background service worker.
- [ ] **Step 2:** `background.ts`: `chrome.cookies.onChanged` listener scoped to `domain === "www.linkedin.com"` and cookie name in `["li_at", "JSESSIONID"]`. On change, read both current cookie values via `chrome.cookies.get`, `fetch("http://127.0.0.1:<port>/session", {method: "POST", body: JSON.stringify({li_at, jsessionid})})`. Wrap the fetch in try/catch — if linkedinclaw's listener isn't running, fail silently and retry on the next cookie change event, never surface an error to the user.
- [ ] **Step 3:** `extension/README.md`: install steps (load unpacked, or pack once stable), and an explicit statement of what it does (reads two cookie values from linkedin.com, POSTs them to localhost only) and does not do (no page scraping, no network access beyond localhost and reading its own cookie jar).
- [ ] **Step 4:** Manual test: install unpacked, log into linkedin.com normally, start `linkedinclaw`'s listener (via `doctor` or a small `linkedinclaw serve-session` command added in this step), confirm `session.json` updates.
- [ ] **Step 5:** Commit: `git add extension && git commit -m "Add companion extension for passive session cookie refresh"`

## Task 12: README rewrite

**Files:**
- Modify: `README.md`

- [ ] **Step 1:** Rewrite full `README.md` following discrawl's structure (Why, What It Does, Requirements, Install, Quick Start, Commands, Not Yet Included, Safety) with linkedinclaw's own-account-only disclaimer prominent near the top, matching the tone of discrawl's "not a selfbot" section.
- [ ] **Step 2:** Commit: `git add README.md && git commit -m "Rewrite README for full crawlkit-based scope"`

---

## Self-Review Notes

- **Spec coverage:** every spec section (auth two-path, sync sources, schema, commands,
  error handling, extension) has a corresponding task above. Rate-limit/backoff from
  spec's "Error handling" section covered in Task 4 (`ErrDeferred`) and Task 5
  (sync catches it and continues).
- **Exact Voyager/export field names are deliberately left as "capture from a real
  account" rather than invented** — LinkedIn's internal API isn't public/stable
  documentation, and guessing field names here would be worse than an explicit
  instruction to record real fixtures once. This is the one place the plan asks the
  executor to do discovery rather than following a spec verbatim; flagged, not hidden.
- **Type consistency check:** `session.Resolve`/`session.Store` signatures (Task 3)
  used consistently by `login` (Task 7) and `doctor` (Task 9). `sync.Run`'s
  `SourceAPI`/`SourceExport`/`SourceBoth` enum (Task 5) is the one flag surface — no
  competing naming introduced later.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-09-linkedinclaw-crawlkit-mirror.md`.

Per your instruction, execution does not go through Claude subagents. Handoff is:

1. I dispatch this plan + the design spec to **glm5.2** (`pi -p -a --tools
   read,grep,find,ls,edit,write,bash`) as orchestrator, in the cloned
   `~/Documents/Personal/linkedinclaw` worktree, instructing it to work task-by-task,
   committing after each, and to shell out to `gemini-personal` for the actual code
   generation on each task.
2. When glm5.2 reports done, I review `git log`/`git diff` myself for architecture
   coherence against the spec.
3. I send the resulting diff to **codex-personal** (`CODEX_HOME=~/.codex-personal`)
   for an independent correctness review.
4. I read codex-personal's report, make the go/no-go call, and handle any
   commit/PR/push myself directly.

Starting step 1 now unless you want to adjust the split first.
