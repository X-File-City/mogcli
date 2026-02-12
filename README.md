# mogcli

Microsoft 365 in your terminal.

`mogcli` is a Microsoft Graph CLI for personal and enterprise workflows. It provides a consistent, scriptable interface for Outlook Mail, Calendar, Contacts, Groups, Tasks, and OneDrive.

## Features

- First-class workloads: Outlook Mail, Calendar, Contacts, Groups, Tasks (Microsoft To Do), and OneDrive.
- Profile-based auth for both consumer and enterprise accounts.
- Single active profile model for predictable command behavior.
- Delegated and app-only auth modes (enterprise app-only).
- Stable `--json` and `--plain` output modes for scripting.
- Built-in retry/backoff and clear, actionable API error output.

## Install

Homebrew:

```bash
brew install mogcli/tap/mog
```

Prebuilt binaries:

- Download from Releases and place `mog` on your `PATH`.

Build from source:

```bash
git clone https://github.com/<your-org>/mogcli.git
cd mogcli
go build -o bin/mog ./cmd/mog
```

## Quick Start

Register two apps in Microsoft Entra ID:

1. Consumer app registration (MSA audience).
2. Enterprise app registration (work/school audience).

Then login profiles:

```bash
# Consumer (MSA)
mog auth login \
  --profile personal \
  --audience consumer \
  --client-id <consumer-client-id>

# Enterprise delegated
mog auth login \
  --profile work \
  --audience enterprise \
  --client-id <enterprise-client-id> \
  --tenant <tenant-id-or-domain>
```

Switch active profile:

```bash
mog auth use personal
mog auth whoami
```

Run commands:

```bash
mog mail list --max 20
mog calendar list --from today --to tomorrow
mog onedrive ls --path /
```

## Auth Model

- Separate consumer and enterprise app registrations.
- Multiple saved profiles.
- Exactly one active profile at a time.
- Profile-isolated token/cache storage.
- Enterprise-only app-only mode.

App-only login example:

```bash
mog auth login \
  --profile work-app \
  --audience enterprise \
  --mode app-only \
  --client-id <enterprise-client-id> \
  --tenant <tenant-id-or-domain> \
  --client-secret-env MOG_CLIENT_SECRET
```

## Commands

Top-level groups:

- `mog auth`
- `mog mail`
- `mog calendar`
- `mog contacts`
- `mog groups`
- `mog tasks`
- `mog onedrive`
- `mog config`

Use `--help` at any level:

```bash
mog --help
mog mail --help
mog onedrive put --help
```

## Examples

Mail:

```bash
mog mail list --query "from:alerts@example.com" --max 50
mog mail get <message-id>
mog mail send --to "dev@contoso.com" --subject "Deploy complete" --body "Finished."
```

Calendar:

```bash
mog calendar list --from 2026-02-12 --to 2026-02-19
mog calendar create --subject "Planning" --start "2026-02-13T16:00:00-08:00" --end "2026-02-13T16:30:00-08:00"
```

Contacts:

```bash
mog contacts list --max 100
mog contacts create --display-name "Jane Doe" --email "jane@contoso.com"
```

Groups:

```bash
mog groups list --max 100
mog groups members <group-id>
```

Tasks:

```bash
mog tasks lists
mog tasks list --list <list-id>
mog tasks complete --list <list-id> --task <task-id>
```

OneDrive:

```bash
mog onedrive ls --path /
mog onedrive put ./report.pdf --path /Reports/report.pdf
mog onedrive get /Reports/report.pdf --out ./report.pdf
```

## Output Modes

Default output is human-readable table/text.

- `--json`: structured JSON output.
- `--plain`: stable parseable plain output (no color).

Examples:

```bash
mog mail list --json | jq '.messages[0]'
mog tasks list --plain
```

## Configuration

Configuration and state are stored under the user config directory (platform-specific), including:

- CLI config
- profile metadata
- keyring/keychain backend settings

Secrets and tokens are stored in OS keychain/keyring when available, with secure fallback behavior for headless environments.

## Permissions and Consent

`mogcli` uses least-privilege scopes per command group. Some Graph capabilities differ by account type or auth mode:

- Groups are enterprise/work accounts only.
- Some endpoints are delegated-only.
- App-only support is limited to workloads/endpoints that Graph allows.

`mog` help output includes required scopes for each command.

## Exit Codes

- `0`: success
- `1`: runtime/API/auth failure
- `2`: usage/argument/parse error

## Troubleshooting

Show active profile and auth state:

```bash
mog auth whoami
mog auth accounts
```

Enable verbose logging:

```bash
mog --verbose mail list
```

Common fixes:

- Re-auth when scopes changed: `mog auth login --profile <name> --force-consent`
- Switch profile: `mog auth use <profile>`
- Reset session for a profile: `mog auth logout --profile <name>`

## Development

```bash
go test ./...
go run ./cmd/mog --help
```

Repository docs:

- `docs/` for architecture, migration, and contributor notes.
- `opensrc/` for local reference sources used during implementation.

## License

MIT
