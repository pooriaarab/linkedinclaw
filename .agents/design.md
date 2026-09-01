# linkedinclaw Design System

This file defines the implemented CLI and terminal explorer language. The project
has no custom production website or browser interface.

## Overview

`linkedinclaw` is a Go command-line tool with plain-text, JSON, and interactive
terminal output. The `tui` command passes archive rows into crawlkit v0.13.4
without product-specific style overrides.

The explorer title is `LinkedIn Claw Explorer`. Its empty state is
`No data loaded yet. Run 'linkedinclaw sync' first.`

Connections, conversations, messages, posts, saved posts, and followed companies
share one row model. The explorer selects its layout from the available row data.

## Colors

The shared TUI uses these exact colors:

- Header: `#0d1321` background with `#f7f7ff` text.
- Main text: `#dfe7ef`.
- Muted text: `#8b95a7`.
- Subtle accent text: `#8fb8d8`.
- Group pane accent: `#5bc0eb`.
- Context pane accent: `#9bc53d`.
- Detail pane accent: `#fde74c`.
- Focused pane border: `#f7f7ff`.
- Focused selection: `#f2c94c` on `#1d1e18`.
- Blurred selection: `#c3b66f` on `#171711`.
- Active row: `#f2c94c` on `#14130f`.
- Inactive row: `#8793a3` on `#0f141b`.
- Local footer: `#5bc0eb` with `#05070d` text.

Crawlkit also defines a `#f2c14e` remote footer. The current `linkedinclaw`
integration does not set a remote source mode, so it uses the local footer.

Outside the TUI, human commands use the terminal's colors. Diagnostics use `✓`
and `✗` as text symbols rather than assigned colors.

## Typography

Use the user's terminal font. The product defines no font family, pixel scale, or
custom line height.

The TUI renders its header and pane titles in bold. Muted labels use the muted
foreground. Table headers use bold pane accent colors.

TUI Markdown keeps heading hierarchy through bold text. It normalizes lists,
quotes, fenced code, inline marks, and links into terminal-safe lines. It wraps
content to the pane width. Horizontal divider rules stop at 72 cells.

JSON output uses two-space indentation. Human output remains line-oriented.

## Layout

The TUI stacks a one-line header, a pane area, and a two-line footer.

The pane area contains group, context, and detail views. It can place all three
side by side, stack all three, or stack detail below two upper panes. Each pane
uses one cell of horizontal padding.

Rendering uses a minimum width of 40 cells. When terminal height is unavailable,
the fallback is 12 rows. Content truncates or wraps within the available cells.

Pane titles use `[*]` for the focused pane and `[ ]` for other panes. The header
reports row counts, filters, sort modes, group mode, layout, and detail mode.
The footer reports status and the available controls.

## Elevation & Depth

The interface is flat. It uses no shadows, gradients, blur, or layered surfaces.

Borders establish pane boundaries. The white focused border moves between panes.
Accent borders identify each pane's role. Selection backgrounds separate the
current row from surrounding rows.

Action menus appear as bordered terminal blocks. Their border and selection
colors create the only overlay-like depth.

## Shapes

Use standard rectangular terminal borders. Do not add rounded corners or vector
decoration.

Rows fill the available pane width. Section rows use bold subtle-accent text.
Tables align data in fixed terminal cells and truncate content when needed.

Keep symbols functional:

- `[*]` and `[ ]` show pane focus.
- `✓` and `✗` show diagnostic results.
- `-` marks sync summary items.
- `>` marks the selected action menu item.

## Components

- Header: title, row count, filter, sort, group, layout, and detail status.
- Group pane: top-level archive groups.
- Context pane: members of the selected group.
- Detail pane: the selected record, help, or action menu.
- Footer: current status on one line and controls on the next.
- Search output: `[kind] urn: snippet`.
- Message output: `sender (timestamp): body`.
- Sync summary: `Synchronization finished!`, category headings, indented lists,
  notes, and discovered GDPR export paths.
- Doctor and diagnostics: one `✓` or `✗` line per check, then `passed/total`.
- JSON output: structured command or row data with two-space indentation.

The TUI exposes keyboard and mouse controls. `Tab` and arrow keys focus panes.
Clicks select rows and headers. Right-click or `a` opens actions. `/` filters,
`#` jumps, and `j` or `k` scrolls. `s`, `m`, and `S` control sorting. `v`, `d`,
and `l` change views. `r` refreshes. `o` opens, `c` copies, `?` shows help, and
`q` quits.

## Do's and Don'ts

- Do keep human output short, stable, and line-oriented.
- Do keep JSON free of human commentary.
- Do preserve exact row kinds and archive terms.
- Do use crawlkit's pane, focus, selection, and footer rules.
- Do ensure important status remains clear without color.
- Do preserve narrow-terminal wrapping and truncation.
- Don't invent a web palette, custom font, logo, or animation system.
- Don't copy LinkedIn branding or imply LinkedIn endorsement.
- Don't add product-specific TUI styling without changing this source.
- Don't expose credentials, cookie values, message bodies, or private archive data
  in examples, screenshots, or fixtures.
