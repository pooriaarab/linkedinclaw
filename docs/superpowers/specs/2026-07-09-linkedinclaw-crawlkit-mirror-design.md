# linkedinclaw: crawlkit-based LinkedIn mirror — design

## Why

`pooriaarab/linkedinclaw` exists as an empty placeholder ("LinkedIn saved/bookmarks
scraper"). `openclaw/discrawl` and `openclaw/slacrawl` establish a family pattern —
Go CLI, `crawlkit`-backed SQLite+FTS5 local archive, git-backed snapshot publishing,
`doctor`/`sync`/`tui`/`search` command surface. This repo repurposes linkedinclaw in
place to join that family: a full personal LinkedIn mirror, not just saved posts.

Scope is explicitly **own-account only**. No multi-account scraping, no anti-detection
tooling, no CAPTCHA automation. This is a personal-data-export tool in the same spirit
as Google Takeout or `yt-dlp` — using your own authenticated session to read your own
data — not a mass-scraping or evasion tool.

## Non-goals (v1)

- Not a Marketplace/public distribution-hardened tool (same caveat slacrawl documents).
- Not multi-account. One `li_at` session per config, matching crawlkit's per-app config
  model.
- No automated email-polling for the GDPR export download link. User downloads the zip
  manually and points `export import` at it.
- No captcha-solving or detection-evasion automation. Login/2FA is completed by the
  user, by hand, in a real browser window.

## Architecture

Go binary depending on `github.com/openclaw/crawlkit`:

- `crawlkit/config` — TOML config, XDG/macOS-native paths (`~/Library/Application
  Support/linkedinclaw/`), token diagnostics.
- `crawlkit/store` — SQLite open/query/FTS5 helpers.
- `crawlkit/state` — sync cursors per data category.
- `crawlkit/mirror` — git-backed archive publish/subscribe (private snapshot repo).
- `crawlkit/tui` — shared terminal explorer (reused as-is, no custom UI work).
- `crawlkit/output`, `crawlkit/control` — text/json output, `metadata --json` for
  `crawlctl` discovery.

linkedinclaw owns: Voyager API client, GDPR export parser, auth/cookie acquisition,
schema, CLI commands — same ownership split discrawl/slacrawl/gitcrawl already use
on top of crawlkit.

A second component, the **companion browser extension** (MV3, TypeScript), lives in
this same repo under `extension/`. It is the passive, ongoing cookie source — the
`wiretap` analog. It does not call any LinkedIn API itself; it only reads the
already-authenticated session's cookies and writes them to a local file the CLI reads.

## Auth: two acquisition paths, one storage location

Both paths land the same two values — `li_at` and `JSESSIONID` — in the OS keyring
(`security add-generic-password -s linkedinclaw -a li_at`), read with the same
env-then-keyring resolution order discrawl uses (`LINKEDINCLAW_LI_AT` env var first).

1. **Bootstrap (`linkedinclaw login`), via `agent-browser` skill.** Interactive,
   one-shot. Drives a real browser to linkedin.com/login. The user completes
   login/2FA/any challenge by hand — the automation does not attempt to solve
   anything adversarial. Once `linkedin.com` shows an authenticated page, the command
   reads `li_at`/`JSESSIONID` from the browser's cookie jar and stores them.
   Re-run manually whenever a full re-login is needed (rare — `li_at` is long-lived,
   ~1 year).

2. **Ongoing refresh, via the companion extension.** Once installed, the extension
   runs in the user's normal, already-logged-in Chrome. It watches for the
   linkedin.com cookie jar and, on change, writes the current `li_at`/`JSESSIONID`
   to a local file (`~/Library/Application Support/linkedinclaw/session.json`,
   0600 perms). `linkedinclaw doctor`/`sync` reads this file if present and newer
   than the keyring value, and promotes it into the keyring. This is what keeps
   `crawlctl`-scheduled unattended sync working long-term without needing an agent
   session to re-run `login`.

Both paths are optional independently: a user who only wants occasional manual syncs
never needs the extension. A user who wants `crawlctl` cron-style sync needs the
extension installed once.

## GDPR export

`linkedinclaw export request` uses `agent-browser` to drive LinkedIn's own Settings →
"Get a copy of your data" flow: select categories, submit. This is the same
interactive/human-in-the-loop shape as login — LinkedIn emails the download link
async, so the command prints "run `export import <path-to-zip>` once you have the
file" and exits. No polling, no Gmail integration in v1.

`export import <zip>` unzips and parses the CSVs into the same tables the API sync
populates (reconciliation, not a separate schema) — mirrors how slacrawl's `wiretap`
source and API source both feed the same `messages` table.

## Sync sources

- `--source api`: Voyager internal API over the cookie session. Fixed conservative
  request-rate cap (default: low double-digit requests/minute) with exponential
  backoff on 429/999. Pulls: profile, 1st-degree connections, conversations +
  messages, own authored posts, saved/bookmarked posts, followed companies.
- `--source export`: GDPR zip import only, no live API calls.
- `--source both` (default): runs `--source api`, then checks the configured
  Downloads folder for an unimported export zip and prompts if found.

`sync --full` forces a complete historical pull (connections list, full message
history per conversation) the way discrawl's `--full` forces complete channel
history. Routine `sync` does incremental-only, using `crawlkit/state` cursors.

## Schema

Tables: `profile`, `connections`, `conversations`, `messages`, `posts` (own-authored),
`saved_posts`, `companies_followed`, plus `crawlkit/state`-managed `sync_state`. FTS5
index over `messages`, `posts`, `saved_posts` body text.

## Commands

`init`, `doctor`, `login`, `sync [--source api|export|both] [--full]`,
`export request`, `export import <zip>`, `search "<query>"`,
`messages --person <name> --hours <n>`, `tui`, `status --json`, `metadata --json`,
`diagnostics --json`, `check-update`, `subscribe <git-url>`, `publish`.

## Error handling

- 429/999 from Voyager API: exponential backoff, bounded retry count, then defer that
  category to next `sync` run (same per-channel-defer pattern discrawl uses for
  pathological channels) — one stuck category never pins the whole run.
- Expired `li_at`: `doctor` and `sync` detect an auth-rejected response and print
  "run `linkedinclaw login`" rather than silently producing an empty archive.
- Malformed/partial GDPR zip: `export import` validates expected CSV files exist
  before writing anything to SQLite; partial zips are rejected with a clear message,
  not partially imported.

## Testing

- Voyager client: unit tests against recorded fixture JSON responses (no live network
  in CI — same reasoning as crawlkit's own test isolation from real app stores).
- Export parser: unit tests against a fixture zip with known CSV contents.
- Schema/store/FTS: integration test using `crawlkit/store` against a temp SQLite DB.
- Extension: manual test only for v1 (cookie-read + file-write, small enough surface
  that an automated test harness isn't worth building yet).

## Delegation split

- **gemini-personal**: Voyager API client (Go), `agent-browser` login/export-request
  drivers, companion MV3 extension (TypeScript).
- **glm5.2 (via `pi`)**: schema/store/CLI command scaffolding, FTS wiring, config
  plumbing (Go) — mechanical, crawlkit-pattern-following work.
- Claude (me): review both diffs, verify build + tests, integrate, keep architecture
  coherent across the two workstreams.
