# AGENTS.md

Instructions for AI coding agents working with this codebase.

<!-- opensrc:start -->

## Source Code Reference

Source code for dependencies is available in `opensrc/` for deeper understanding of implementation details.

See `opensrc/sources.json` for the list of available packages and their versions.

Use this source code when you need to understand how a package works internally, not just its types/interface.

### Fetching Additional Source Code

To fetch source code for a package or repository you need to understand, run:

```bash
npx opensrc <package>           # npm package (e.g., npx opensrc zod)
npx opensrc pypi:<package>      # Python package (e.g., npx opensrc pypi:requests)
npx opensrc crates:<package>    # Rust crate (e.g., npx opensrc crates:serde)
npx opensrc <owner>/<repo>      # GitHub repo (e.g., npx opensrc vercel/ai)
```

<!-- opensrc:end -->

## Implementation Memory (Keep This Aligned)

Use this section as the source of truth for implementation decisions unless the user overrides them.

### Canonical Docs

- Primary implementation spec: `docs/microsoft-port-plan.md`
- Product-level README target state: `README.md`

If you make architecture or product-surface changes, update these docs in the same PR.

### Locked Product Decisions

- This project is a Microsoft-focused CLI named `mog` / `mogcli`.
- Auth must support both consumer (MSA) and enterprise (Entra ID) users.
- Use separate app registrations/client IDs for consumer and enterprise audiences.
- Users can have multiple profiles, but exactly one active profile at a time.
- Delegated auth/workloads ship first.
- App-only auth is phase 2 (enterprise-only), after delegated flows are stable.

### Scope Priorities

Implement these workloads first, in this order unless user overrides:

1. OneDrive
2. Mail
3. Calendar
4. Contacts
5. Tasks (Microsoft To Do)
6. Groups

### Command UX Conventions

- Prefer noun-grouped commands.
- `mog auth ...`
- `mog mail list|get|send`
- `mog calendar list|get|create|update|delete`
- `mog contacts list|get|create|update|delete`
- `mog groups list|get|members`
- `mog tasks lists|list|get|create|update|complete|delete`
- `mog onedrive ls|get|put|mkdir|rm`
- Preserve stable scripting outputs (`--json`, `--plain`) and keep help text explicit about required scopes.

### Porting Rules (from `opensrc/repos/github.com/steipete/gogcli`)

- Reuse shell/basics where practical.
- parser/root/help/output/ui/error/usage/exit plumbing
- config + alias/client mapping plumbing
- secrets/keyring integration
- retry/backoff/circuit-breaker primitives
- test harness utilities
- Preserve upstream license notices/attribution when copying substantial code blocks.
- Replace Google-specific modules entirely.
- auth flows (`internal/googleauth/*`)
- Google API client constructors and service bindings
- service-specific command implementations

### Microsoft Graph Constraints (Must Be Encoded in UX)

- Groups are enterprise/work-account features; personal Microsoft accounts are unsupported.
- Group calendar behavior differs and has delegated-only constraints.
- Some Tasks app-only write scenarios are unsupported.
- Some OneDrive endpoints are delegated-only; app-only support is endpoint-specific.
- Surface unsupported combinations as explicit, actionable errors (never ambiguous failures).

### Security and Reliability Requirements

- Prefer keychain/keyring-backed token storage; avoid plaintext token persistence by default.
- Isolate token/cache material by profile/account to prevent cross-profile token reuse.
- Detect account mismatch on restored tokens and invalidate mismatched cache entries.
- Implement bounded retry with `Retry-After` handling and jitter; avoid unbounded recursive retries.
- Normalize/sanitize AADSTS and Graph errors for user-facing output.

### Decision Hygiene

- Treat `docs/microsoft-port-plan.md` as the canonical backlog and decision log.
- If an implementation detail is still marked open in that doc, ask the user before hard-coding it.

### Testing Expectations

- Add unit tests for parser/help/output/auth/profile invariants.
- Add HTTP contract tests for Graph pagination, throttling, and error mapping.
- Keep an integration matrix in mind.
- MSA delegated
- Entra delegated
- Entra app-only (phase 2+)
- For known unsupported scenarios (for example MSA + groups), add explicit failure tests.
