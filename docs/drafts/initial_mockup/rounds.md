# Grill sessions log

Design tree for the grill-chef MCP server. One entry per round; answers marked
with the branch tags they settle.

## Round 1 — foundation branches

Questions served: session ownership (Q1.1), URL handoff (Q1.2), rendering
strategy (Q1.3), stack (Q1.4), skill layering (Q1.5).

**Settled:**

- [session] Create-and-wait: the MCP tool call blocks until the user submits
  round data (screen E); server owns all state; agent receives answers as the
  tool result. Open sub-issue: how session ids embed in URLs and how the user
  reaches the right one when multiple agents are calling.
- [mcp] Session handoff: assume localhost, auth-free sessions for now. UX rule:
  server tries auto-open of the browser and also returns the URL as a fallback
  in the tool result. Non-static (dynamic) port.
- [stack] Rendering: server-rendered HTML + small progressive enhancement
  (vanilla JS only).
- [stack] Runtime: Go, single self-contained binary, stdlib templates, as few
  external deps as possible; official Go MCP SDK accepted as a dependency.
- [skill] Separate skill `grill-me-json` that is standalone — exactly
  reproduces grill-me semantics (tree/frontier/caps/recap/legend) except the
  output medium (JSON via MCP instead of markdown text). No reference to, and
  no fallback on, the markdown grill-me skill.

**Remaining frontier (retained, 14):** session id mapping/multi-agent
sessions; MCP transport; long-running call vs client timeouts; JSON schema
shape; answer semantics (choice/comments/Other); recap + legend screens;
multi-session concurrency; port configuration; read-only history enforcement;
theme; static assets; naming; fallback when browser unavailable; closing flow.

## Round 2

(next round goes here)
