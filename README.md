# linkedinclaw

🔗 LinkedIn saved/bookmarks scraper. Stores your LinkedIn saves nicely claw-able for agents.

## Scope & Disclaimer

`pooriaarab/linkedinclaw` exists as an empty placeholder ("LinkedIn saved/bookmarks
scraper"). `openclaw/discrawl` and `openclaw/slacrawl` establish a family pattern —
Go CLI, `crawlkit`-backed SQLite+FTS5 local archive, git-backed snapshot publishing,
`doctor`/`sync`/`tui`/`search` command surface. This repo repurposes linkedinclaw in
place to join that family: a full personal LinkedIn mirror, not just saved posts.

Scope is explicitly **own-account only**. No multi-account scraping, no anti-detection
tooling, no CAPTCHA automation. This is a personal-data-export tool in the same spirit
as Google Takeout or `yt-dlp` — using your own authenticated session to read your own
data — not a mass-scraping or evasion tool.

### Non-goals (v1)

- Not a Marketplace/public distribution-hardened tool (same caveat slacrawl documents).
- Not multi-account. One `li_at` session per config, matching crawlkit's per-app config
  model.
- No automated email-polling for the GDPR export download link. User downloads the zip
  manually and points `export import` at it.
- No captcha-solving or detection-evasion automation. Login/2FA is completed by the
  user, by hand, in a real browser window.
