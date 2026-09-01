# linkedinclaw Brand

`linkedinclaw` mirrors a person's own LinkedIn data into a local, searchable
archive. The exact product name is lowercase and set in code formatting.

## Purpose

Help one person search and preserve their own profile, connections, messages,
posts, saved posts, and followed companies.

The product supports two sources: an authenticated Voyager session and a LinkedIn
GDPR export. It can publish archive snapshots to a private Git repository.

## Audience

Write for developers and technical users who understand terminals, SQLite,
environment variables, keyrings, cookies, and Git repositories.

## Scope

Describe `linkedinclaw` as an own-account export and archive tool. Never describe
it as a mass scraper, outreach tool, account farm, or challenge bypass.

Users complete sign-in, two-factor prompts, and challenges themselves. The
automation must not claim to solve them.

`linkedinclaw` is not affiliated with or endorsed by LinkedIn. Do not use
LinkedIn logos, corporate styling, or a blue palette to imply that relationship.
This repository defines no `linkedinclaw` logo.

## Voice

- Use plain, technical language.
- Lead with the action or result.
- Name the source, destination, and access requirement.
- Distinguish implemented behavior from planned work.
- State failure and deferred work directly.
- Avoid hype, surveillance language, and broad security claims.

## Product language

Use these terms consistently:

- `archive` for the mirrored data set.
- `sync` for a Voyager or export refresh.
- `GDPR export` for the user-downloaded LinkedIn archive.
- `snapshot` for a private Git-backed archive copy.
- `session credentials` for `li_at` and `JSESSIONID` together.
- `connection`, `conversation`, `message`, `post`, and `saved post` for rows.

## Output

Human output is short, line-oriented terminal text. Use clear headings, two-space
indented lists, direct errors, and a final result.

Machine output is JSON. Keep field names stable and use two-space indentation.
Do not mix commentary into JSON output.

The terminal explorer uses the shared crawlkit visual system. Treat its colors,
panes, focus cues, and controls as part of the current product experience.

## Privacy boundary

Mirrored records live in the local SQLite archive. Optional publishing sends a
snapshot to the private Git remote that the user configures.

Credential resolution checks environment variables first. On macOS, it can read
and write the operating system keyring.

The companion extension reads only the `li_at` and `JSESSIONID` cookies. Its code
posts them to `http://127.0.0.1:9090/session` and does not access page content.
The listener binds to `127.0.0.1`, writes a mode-0600 session file, and attempts
keyring storage.

Describe these controls precisely. Do not convert them into absolute privacy or
security guarantees.
