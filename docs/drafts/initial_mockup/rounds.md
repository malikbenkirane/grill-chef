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

## Round 2 — seam between agent and browser

Questions served: transport + global state (Q2.1), durability/recovery of the
blocking wait (Q2.2), session id minting and URL shape (Q2.3), payload
granularity across rounds (Q2.4), tool result content and answer immutability
(Q2.5).

**Settled:**

- [mcp] Global state on disk: state database under
  `os.UserCacheDir() + "/grill-chef/state.db"` so restart, crash or tool
  timeout can restore a grill.
- [mcp] Transport: MCP over stdio. The browser HTTP server is NOT something the
  user starts: it lazily starts if not already running, and this lifecycle
  design is explicitly deferred to round 3 discussion.
- [session] Session id: minted by a first MCP tool call that starts the grill
  session (before round 1, not per round); the agent keeps the id in its chat
  history and passes it on follow-up calls; stateless handlers + global cache
  let state be restored from that id alone.
- [session] Wild stalls are expected: a grill station can sit idle for hours or
  days. If a tool call times out, the agent (or user asking) can call a
  recovery-style tool: it checks the cache for pending answers, and if none are
  submitted, it re-opens the browser for that session — same drill as first
  time.
- [session] URL namespacing: `/s/<id>/` prefix; ambiguity dismissed. Sidebar
  may additionally list pending sessions, a session history at the bottom, and
  allow dismissing (clearing) sessions.
- [caps/frontier] No planned-round placeholders in the sidebar: grill-me's
  frontier is answer-dependent, and placeholder rounds would add noise and
  bias from hypothetical guesses.
- [schema/result] Clarification: "comments read-only" means immutable.
  Agent-side state responsibility: the agent's chat history already holds each
  round's answers; full transcript echo in the tool result is undecided
  (deferred; leaning no).

**Remaining frontier (retained, ~9):** HTTP auto-start lifecycle take-back;
tool surface shape (create/get/wait tools, naming); field-by-field JSON
schema; widget semantics (Other, comments, editing, skip); sidebar scope
(global session index vs per-session rounds nav); recap/legend screens;
port/config; read-only enforcement; theme; naming; closing flow.
