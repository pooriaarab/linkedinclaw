# linkedinclaw 🔗 — Mirror your own LinkedIn account into SQLite; search it locally

`linkedinclaw` mirrors your own LinkedIn account into local SQLite so you can search
your profile, connections, conversations, own posts, saved posts, and followed
companies without depending on LinkedIn's search. It can also import a LinkedIn GDPR
data export and publish the archive as a private Git snapshot repo, so other machines
get a read-only copy without LinkedIn credentials.

There are two data sources:

- Voyager internal API sync over your own authenticated cookie session for profile,
  1st-degree connections, conversations + messages, own authored posts,
  saved/bookmarked posts, and followed companies
- LinkedIn GDPR export import for the data LinkedIn lets you download as a zip

Scope is explicitly **own-account only**. This is a personal-data-export tool in the
same spirit as Google Takeout or `yt-dlp` — using your own authenticated session to
read your own data — not a mass-scraping or evasion tool. See [Safety](#safety).

## What It Does

- pulls your profile, 1st-degree connections, conversations, and messages from the
  Voyager internal API over your own cookie session
- pulls your own authored posts, saved/bookmarked posts, and followed companies
- imports a LinkedIn GDPR data export zip into the same tables the API sync populates
  (reconciliation, not a separate schema)
- maintains FTS5 search indexes over message, post, and saved-post body text for fast
  local search
- publishes and imports private Git-backed archive snapshots for read-only access from
  other machines
- browses the full local archive in a shared terminal explorer UI
- exposes `status`, `metadata`, `diagnostics`, and `doctor` for local launchers,
  automation, and `crawlctl` discovery
- runs a fixed conservative request-rate cap (20 requests/minute) with exponential
  backoff on Voyager `429`/`999`, deferring a stuck category instead of aborting the
  whole sync
- keeps per-category incremental cursors so routine `sync` only fetches new data;
  `sync --full` forces a complete historical pull

`sync` defaults to `--source both`: it runs the Voyager API sync, then scans your
Downloads folder for an unimported GDPR export zip and reports it. Use `--source api`
for API-only or `--source export` for zip-only import.

## Requirements

- Go `1.26+`
- a LinkedIn account you can sign into by hand, plus its `li_at` and `JSESSIONID`
  session cookies
- the `agent-browser` skill/CLI on `PATH` for `linkedinclaw login` and
  `linkedinclaw export request`, which drive a real Chrome profile
- optional: the companion Chrome extension under `extension/` for passive cookie
  refresh, so scheduled syncs keep working long-term without re-running `login`

### Session credentials

Token resolution (env first, then OS keyring):

1. `LINKEDINCLAW_LI_AT` and `LINKEDINCLAW_JSESSIONID` env vars (both must be set)
2. OS keyring item `linkedinclaw` / `li_at` and `linkedinclaw` / `jsessionid`

> The keyring path currently shells out to the macOS `security` command, so it is
> **macOS-only** for now. On other OSes, set both env vars.

Fastest path:

```bash
linkedinclaw login      # opens Chrome, you sign in + complete 2FA by hand
linkedinclaw doctor     # shows where credentials were resolved from
```

Or store them directly in the keyring:

```bash
security add-generic-password -U -s linkedinclaw -a li_at      -w "$LINKEDINCLAW_LI_AT"
security add-generic-password -U -s linkedinclaw -a jsessionid -w "$LINKEDINCLAW_JSESSIONID"
```

`li_at` is long-lived (roughly a year); `JSESSIONID` rotates, which is what the
companion extension keeps fresh.

Default runtime paths follow OS conventions via crawlkit. On macOS everything lives
under `~/Library/Application Support/linkedinclaw/` (database, config, logs), and the
companion extension's `session.json` fallback is written there too (mode `0600`).

## Install

Not published or Homebrew-tapped yet. Build from source:

```bash
git clone https://github.com/pooriaarab/linkedinclaw.git
cd linkedinclaw
go build -o bin/linkedinclaw ./cmd/linkedinclaw
./bin/linkedinclaw --version
```

Examples below assume `linkedinclaw` is on `PATH`. If you built from source without
installing it, replace `linkedinclaw` with `./bin/linkedinclaw`.

## Quick Start

```bash
linkedinclaw login              # open Chrome and complete sign-in + 2FA by hand
linkedinclaw doctor             # config, credentials, Voyager auth, DB + FTS
linkedinclaw sync               # pull profile, connections, messages, posts, ...
linkedinclaw search "onboarding"
linkedinclaw tui
```

`login` opens LinkedIn in a real Chrome profile via `agent-browser`, waits for you to
finish signing in, then reads `li_at`/`JSESSIONID` from the browser cookie jar and
stores them in the keyring. `doctor` is the fastest sanity check: it confirms config
loads, shows where credentials came from, verifies a lightweight authenticated Voyager
call, and checks DB + FTS wiring.

Import a GDPR export instead of (or alongside) the API:

```bash
linkedinclaw export import ~/Downloads/Complete_LinkedInDataExport.zip
```

Git-only reader setup on another machine, no LinkedIn credentials required:

```bash
linkedinclaw mirror subscribe https://github.com/example/linkedin-archive.git
linkedinclaw search "onboarding"
```

## Commands

### `login`

Opens LinkedIn in a real Chrome profile via the `agent-browser` skill and waits for you
to complete sign-in and any 2FA/challenge by hand. Once `linkedin.com` shows an
authenticated page, it reads `li_at`/`JSESSIONID` from the browser cookie jar and
stores them in the OS keyring.

```bash
linkedinclaw login
```

The automation never solves a login challenge — you do. If the cookies are missing when
you press Enter, it prints `login not detected -- run linkedinclaw login again once
you've finished signing in.` Re-run after you finish signing in. (`li_at` is long-lived,
so this is rare — roughly once a year.)

### `sync`

Refreshes SQLite from one or both archive sources.

```bash
linkedinclaw sync
linkedinclaw sync --source api
linkedinclaw sync --source export
linkedinclaw sync --source both
linkedinclaw sync --full
linkedinclaw sync --json
```

Sources:

| Source | Reads from | Stores |
| --- | --- | --- |
| `api` | Voyager internal API over your cookie session | profile, connections, conversations, messages, own posts, saved posts, followed companies |
| `export` | a local GDPR export zip only | connections, conversations, messages (tagged `source='export'`); no live API calls |
| `both` | Voyager API, then a Downloads-folder scan for an unimported zip | API data, plus it reports any unimported export zip it finds |

`--full` bypasses the incremental cursor and refetches full history per category (full
connections list, full message history per conversation). Routine `sync` is
incremental-only via per-category cursors.

The Voyager client runs a fixed rate cap of 20 requests/minute with exponential backoff
on `429`/`999` (up to 5 attempts). When retries are exhausted it defers that category to
the next run and continues the rest — one stuck category never pins the whole sync.
Deferred categories and found export zips are reported in the run summary.

`--source export` needs no LinkedIn credentials, so it works without `login`.

### `export`

`export import <zip>` parses a LinkedIn GDPR export zip and upserts its rows into the
same tables the API sync populates, tagging rows `source='export'` where the API does
not also provide them. It validates that the expected CSV files exist before writing
anything, so a partial or malformed zip is rejected, not partially imported.

```bash
linkedinclaw export import ~/Downloads/Complete_LinkedInDataExport.zip
```

`export request` drives LinkedIn's Settings → "Get a copy of your data" flow via the
`agent-browser` skill. LinkedIn emails the download link asynchronously, so after
submitting it prints a reminder to run `export import <path-to-zip>` once you have the
file. There is no email polling.

```bash
linkedinclaw export request
```

> The `agent-browser` driver for `export request` is not wired up yet; for now it points
> you at the manual request URL. `export import` is fully implemented.

### `search`

Searches message, post, and saved-post body text. FTS5 is the only mode.

```bash
linkedinclaw search "onboarding"
linkedinclaw search --json "recruiter reach"
```

Runs three FTS5 `MATCH` queries across `messages_fts`, `posts_fts`, and
`saved_posts_fts`, unions them in application code, and returns matches ranked by FTS5
`rank` with a snippet. Plain-text output is `[kind] urn: snippet`.

### `messages`

Lists messages filtered by participant and time window.

```bash
linkedinclaw messages --person Jane --hours 24
linkedinclaw messages --person Smith
linkedinclaw messages --hours 6
```

Notes:

- `--person` matches a connection's first name, last name, or full name (`LIKE`)
- `--hours` limits to messages within the last N hours; `0`/omitted means no time limit
- joins `messages` to `conversations` and `connections`; a sender with no stored
  connection row falls back to its sender URN

### `tui`

Opens the shared crawlkit terminal explorer over the full local archive.

```bash
linkedinclaw tui
```

Connections, conversations, messages, own posts, saved posts, and followed companies are
all mapped into the explorer's lanes. It reuses crawlkit's explorer UI as-is.

### `mirror`

Git-backed archive publish/subscribe via crawlkit's mirror package.

Publisher (needs a local archive built by `sync`/`export import` first):

```bash
linkedinclaw mirror publish --remote https://github.com/example/linkedin-archive.git
linkedinclaw mirror publish            # --remote is remembered after subscribe/first publish
```

Subscriber (read-only, no LinkedIn credentials required):

```bash
linkedinclaw mirror subscribe https://github.com/example/linkedin-archive.git
linkedinclaw search "onboarding"
```

`subscribe` clones/pulls the snapshot into the archive directory and writes a marker
that puts this instance into read-only subscribe mode. `publish` copies the local
SQLite database into the archive directory, cleans `-wal`/`-shm` sidecars, commits, and
pushes only when there are actual changes.

### `doctor`

Checks config, credentials, Voyager auth, and DB + FTS wiring.

```bash
linkedinclaw doctor
```

Reports the credential source (env vars vs keyring) and verifies all three FTS tables
exist, then prints a `passed/total` summary. An auth-rejected response tells you to
`run linkedinclaw login` rather than silently producing an empty archive.

### `status`

Shows local archive status and item counts.

```bash
linkedinclaw status
linkedinclaw status --json
```

Reports counts for connections, messages, posts, saved posts, and companies followed,
plus the database path and size.

### `metadata`

Prints the application manifest `crawlctl` uses for discovery.

```bash
linkedinclaw metadata --json
```

### `diagnostics`

Runs the same checks as `doctor` but as structured pass/fail output for automation and
CI.

```bash
linkedinclaw diagnostics --json
```

### `check-update`

Checks GitHub for a newer linkedinclaw release.

```bash
linkedinclaw check-update
```

## Safety

Scope is explicitly **own-account only**. This is a personal-data-export tool in the
same spirit as Google Takeout or `yt-dlp` — using your own authenticated session to
read your own data — not a mass-scraping or evasion tool.

`linkedinclaw` does **not**:

- scrape multiple accounts. One `li_at` session per config, matching crawlkit's
  per-app config model.
- use anti-detection tooling or detection-evasion automation. There is no CAPTCHA
  solving, no stealth, no headless evasion.
- automate login or 2FA. `linkedinclaw login` opens a real browser window; you complete
  sign-in and any challenge by hand. The automation only reads the resulting session
  cookies once `linkedin.com` shows an authenticated page.
- poll email for the GDPR export download link. You download the zip by hand and point
  `export import` at it.
- store or print raw cookie values in logs. `doctor` reports *where* credentials were
  resolved from, not their contents.

The companion extension reads only the two `linkedin.com` session cookies and POSTs
them to `http://127.0.0.1` on your machine. It does no page scraping and makes no
remote network requests. See [`extension/README.md`](extension/README.md).

## Not Yet Included

- **Marketplace/public distribution hardening** — same caveat the sibling `slacrawl`
  tool documents. This is a personal tool, not a packaged, distribution-hardened
  product.
- **Multi-account support** — one `li_at` session per config, matching crawlkit's
  per-app config model.
- **Automated email polling for the GDPR export download link** — LinkedIn emails the
  link asynchronously; for now you download the zip manually and point
  `export import` at it.

## Companion extension

The optional Chrome extension under `extension/` is the passive, ongoing cookie source
— the `wiretap` analog. Once installed in your normal, already-logged-in browser, it
watches the `linkedin.com` cookie jar and, on change, writes the current
`li_at`/`JSESSIONID` to a local listener so `crawlctl`-scheduled unattended syncs keep
working long-term without an agent session re-running `login`. It is fully optional: a
user who only wants occasional manual syncs never needs it. Install steps and the
explicit list of what it does and does not do are in
[`extension/README.md`](extension/README.md).
