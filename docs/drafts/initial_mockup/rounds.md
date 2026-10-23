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

## Round 3 — tools, schema, widgets, sidebar

Questions served: HTTP listener ownership (Q3.1), tool surface (Q3.2),
field-level schema (Q3.3), widget semantics (Q3.4), sidebar scope (Q3.5).

**Settled:**

- [lifecycle] Direction: HTTP listener should OUTLIVE the MCP client (detached
  daemon), unlike the single-owner recommendation. Exact detach mechanism
  deferred to round 4.
- [mcp/tool] Tools kept apart, verb-first: `start_grill`, `drive_round`,
  `recover_grill`. User asked to re-check against
  `~/dev/superpowers/skills/writing-skills` guidelines for an updated
  recommendation (done: see Round 4 Q4.2).
- [schema] `drive_round` sends only the current round's JSON; previously driven
  rounds live in global state; screens render FROM STATE. Formatting shortcut:
  essential blocks transferred as markdown; server translates markdown → HTML
  at render time; leaves room for skill-side copy changes without server
  schema churn.
- [caps] Rounds capped at 5; rounds keep arriving while frontier is unresolved.
- [session] `start_grill` accepts a session name used for in-browser rendering
  (session denomination, tab/heading).
- [answers/widgets] Review model: choices/comments reviewed on submit screen;
  ongoing round's questions reachable/enumerated from sidebar; multi-comment
  list with add/remove; a choice is mandatory (no default, no prefill), exactly
  one choice; zero comments is OK except "Other" which requires at least one
  comment; submission requires all questions answered; "Next" enabled only
  once a choice is made; gray "Skip" button sits next to Next; back navigation
  via sidebar index. If not too complex: debounced autosave with a
  disappearing "Saved!" indicator next to the top-right buttons.
- [screens] Sidebar scope settled as per-grill rounds only; global list /
  history / dismiss live in a station home route `/`.
> Note: user's Q3.4 text effectively chose a strict-plus-skip variant; the
> skip flag from Q1 result plan survives.

**Remaining frontier (retained, ~6):** daemon detach mechanics; naming +
writing-skills-informed skill testing; recap/legend screen shape; closing flow
after frontier empty; repo/binary naming + config (port range, theme);
read-only enforcement beyond submit-immutability.

## Round 4 — daemon, tools, schema boundary, closing flow, packaging

Questions served: detach mechanics (Q4.1), naming + testing gate (Q4.2),
structured vs markdown boundary (Q4.3), closing flow (Q4.4), naming/config
(Q4.5).

**Settled:**

- [lifecycle] Detached daemon child; state db records pid + port; recovery
  tools health-check via pid and respawn if stale. User reinforced: the
  server's lifespan is fully decoupled from the MCP client; the agent can only
  (re)start it, and has NO capability to stop it. (The `stop` affordance is a
  human CLI command, never an MCP tool.) Flag for round 5: tailwind dep
  tension and stop-scope confirmation.
- [mcp] Tools renamed with namespace prefix: `grill_start`, `grill_round`,
  `grill_recover`; `grill_recover` auto-opens the browser. Skill work gated by
  writing-skills Iron Law: baseline pressure scenarios without the skill →
  write skill → re-test under pressure → close loopholes.
- [schema] Structured envelope + markdown inline: question round number, ids,
  titles, options list (with option ids), `recommended_index`, reminders tags
  stay STRUCTURAL; option label text, abstract paragraphs, intro, recap prose
  and comments are markdown blobs. Radio buttons bind structurally to option
  ids.
- [closing] Final optional `legend` payload sent by a last `grill_round` call;
  browser renders a legend screen; the shared-understanding confirmation
  stays exclusively in chat (grill-me rule preserved). Closed sessions are
  archived in the station home.
- [dist] Single binary `grill-chef`, subcommands: `mcp` (stdio default),
  `web` (manual daemon start), `stop` (human-only), `ui` (open station home).
  No config file; flags + env only. Skill sources live in this repo under
  `skills/grill-me-json/`. Theming: Tailwind CSS with dark/light mode switch.

past-round read-only enforcement (likely settled
by state-based rendering).

## Round 5 — closing round

Questions served: stop-scope (Q5.1), Tailwind dependency policy (Q5.2),
past-round immutability (Q5.3). All three confirmed via the round recap being
returned unchanged (recommendations accepted as written).

**Settled:**

- [lifecycle] Stop is CLI-only (`grill-chef stop`), never an MCP tool, now or
  later; an explicit "no stop tool" clause goes into the skill prose. The
  agent can only idempotently (re)start the daemon.
- [theme] Tailwind is a build-time asset only: Tailwind standalone CLI
  (vendored under `bin/tailwind`, Makefile-managed) generates static CSS that
  is committed; zero new Go dependencies; self-contained binary promise
  preserved. Theme (dark/light) persists per browser via localStorage.
- [screens] Past-round routes render immutable by construction (pure render
  from answer state, no edit widgets); the current round is the only editable
  surface with internal back-nav. Copy-to-clipboard and JSON snapshot buttons
  remain possible UI-stage niceties that do not reopen the tree.

**Tree closed. Legend follows in the session transcript.**
