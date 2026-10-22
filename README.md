# gh-claimcheck

**Check that a GitHub issue is really free before you start working on it.**

`gh-claimcheck` is a [GitHub CLI](https://cli.github.com) extension for open source contributors. Give it an issue, and it tells you if someone already has an open pull request for it, if someone claimed it in the comments, if the project moved somewhere else, or if maintainers don't merge outside PRs. You get a clear **GO**, **CAUTION** or **SKIP** in a few seconds, so you don't spend an evening on a fix that turns out to be a duplicate.

It can also **find** open, unassigned issues with no linked PR in the languages and projects you care about, and check each one for you.

```console
$ gh claimcheck date-fns/tz#80
date-fns/tz#80  tzOffset: wrong sign for offsets between -01:00 and 00:00
https://github.com/date-fns/tz/issues/80

  ! An open PR looks like the same fix
    date-fns/date-fns#4314 by @contactjawad: Fix tzOffset returning the wrong sign for sub-hour western offsets
    https://github.com/date-fns/date-fns/pull/4314
  ! An open PR looks like the same fix
    date-fns/date-fns#4279 by @Kropiunig: fix(tz): correct tzOffset sign for negative offsets of less than an...
    https://github.com/date-fns/date-fns/pull/4279
  ! Project has moved to date-fns/date-fns
    Pull requests probably belong in the new home. Its pull requests were searched too.
  ! The last outside PR was merged 14 months ago
    6 of the last 6 merged PRs came from 6 outside contributors. Reviews may be slow, or the project may be quiet.
  ✓ Issue is open
  ✓ Nobody is assigned
  · 3 PRs share keywords with the title
    date-fns/date-fns#4396 (open), date-fns/date-fns#4335 (open), date-fns/date-fns#4356 (open). Probably different fixes, but skim them.

  CAUTION  An open PR looks like the same fix
  Look into the points above, and ask on the issue before you start.
```

None of those PRs mention the issue, and they live in a different repository, so the issue page itself shows nothing. That's exactly the case claimcheck is built to catch. (Output trimmed a little; the README notice line is shortened.)

## Install

You need the [GitHub CLI](https://cli.github.com) and a login (`gh auth login`). Then:

```sh
gh extension install RagedPanda/gh-claimcheck
```

Upgrade later with `gh extension upgrade claimcheck`. Prebuilt binaries are provided for macOS, Linux and Windows; you don't need Go.

## Quick start (new to open source? start here)

1. Find an issue you'd like to fix, or let claimcheck find some:

   ```sh
   gh claimcheck find --good-first-issue --lang python
   ```

2. Check the one you picked. A URL, `owner/repo#123`, or just `123` inside a clone all work:

   ```sh
   gh claimcheck https://github.com/owner/repo/issues/123
   ```

3. Read the verdict:

   | Verdict | Meaning | What to do |
   |---|---|---|
   | **GO** | Nobody seems to be on it. | Comment on the issue that you're working on it, then open a PR. |
   | **CAUTION** | Something needs a look: a PR that may be the same fix, an old claim, a maintainer who pushed back, a project that rarely merges outside PRs. | Read the `!` lines. Ask on the issue before you start. |
   | **SKIP** | Someone already has an open or merged PR for it, someone claimed it recently, it's assigned, closed, locked or archived. | Pick another issue. |

4. Before you open your PR, read the project's contributing guide. claimcheck tells you where it is and whether the project has rules about AI-written code.

## What it checks

| Check | Why it matters |
|---|---|
| **Linked and referencing pull requests** (open, draft, merged, closed) | The most common way to waste effort is fixing something that already has a PR. |
| **PRs with a similar title**, ranked by how closely they match | Many PRs never mention the issue number. |
| **Project moved** (README notice like "moved to the monorepo") | The fix and the other PRs are often in the new repository; it is searched too. |
| **Claims in comments** ("I'd like to work on this", `/assign`, "I've opened a PR") | Someone may be on it without being assigned. Recent claims block; old ones only warn. |
| **Assignees, closed, locked, archived** | The basics, all in one place. |
| **Maintainer pushback** ("working as intended", "won't fix", "duplicate of") | A PR for an issue maintainers rejected won't be merged. |
| **Blocking labels** (`needs repro`, `wontfix`, `blocked`, `needs discussion`...) and welcoming ones (`good first issue`, `help wanted`) | Labels say whether a PR is wanted right now. |
| **Outside merges**: who wrote the last 40 merged PRs, and when | Some projects only merge their own team's work, or haven't merged anything in a year. |
| **AI policy** in `CONTRIBUTING.md`, `AI_POLICY.md`, `AGENTS.md` | Some projects ban or require disclosure of AI-assisted PRs. |
| **Contributing guide, PR template, CLA** | So you know what the PR needs before you write it. |

Your own claims, assignments and PRs never count against you. If you're already on an issue, it says so.

## Find issues to work on

`gh claimcheck find` searches for **open issues with no assignee, no linked PR, few comments** and a recent creation date, in non-archived repositories. Add `--check` to run the full check on every result.

```sh
# Beginner-friendly issues in Go
gh claimcheck find --good-first-issue --lang go

# Recent issues in popular TypeScript projects that came out GO
gh claimcheck find --lang typescript --min-stars 1000 --check --go-only

# Everything new in one organisation in the last two weeks
gh claimcheck find --org vitejs --days 14 --check

# Search words, inside one repository, with no comments yet
gh claimcheck find "timezone" --repo date-fns/date-fns --max-comments 0
```

```console
$ gh claimcheck find --lang typescript --min-stars 1000 --days 14 --check --limit 6   # trimmed
GO       NativeScript/NativeScript#11566    ★25.7k  0c  today  @nativescript/vite: link check rejects valid aliased default exports
CAUTION  mebjas/html5-qrcode#1076           ★6.2k   0c  today  Bug Iphone14
GO       ranxianglei/billion-context#4307   ★2.4k   0c  today  Compression calls with lost ACKs are permanently stuck in context: client-sid...
CAUTION  anomalyco/opencode#58123           ★220.0k 1c  today  Stuck on a single message from few minutes
```

| Flag | Default | Meaning |
|---|---|---|
| `-R, --repo owner/repo` | | Search one repository (repeatable, or comma separated) |
| `--org name` | | Search an organisation or user (repeatable) |
| `-l, --lang language` | | Only this language (repeatable) |
| `--label name` | | Require a label (repeatable) |
| `--good-first-issue`, `--help-wanted` | | Shortcuts for those labels |
| `--days n` | `60` | Created in the last n days; `0` for any age |
| `--max-comments n` | `3` | At most n comments; `-1` for any |
| `-s, --min-stars n` | | Only repositories with at least n stars |
| `-L, --limit n` | `20` | Number of results, 1 to 100 |
| `-c, --check` | | Run the full check on each result |
| `--go-only` | | Show only results that come out GO (implies `--check`) |
| `--json` | | JSON output |

## For scripts and power users

Check several issues at once, in parallel:

```sh
gh claimcheck cli/cli#14703 vitest-dev/eslint-plugin-vitest#980 date-fns/tz#80 --brief
```

```console
SKIP     cli/cli#14703                        An open PR already references it
GO       vitest-dev/eslint-plugin-vitest#980  You are already on it
CAUTION  date-fns/tz#80                       An open PR looks like the same fix
```

**Exit status.** With `--exit-status`, the exit code tells you the worst verdict: `0` all GO, `3` any CAUTION, `2` any SKIP, `1` an error. Without it, the code is `0` unless something failed.

```sh
gh claimcheck "$issue" --exit-status >/dev/null && echo "free to take"
```

**JSON.** `--json` prints every finding with its level (`block`, `warn`, `pass`, `info`), title, detail and URL. It works with `jq`:

```sh
# Only the issues that are GO
gh claimcheck find --lang rust --check --json | jq -r '.[] | select(.report.verdict == "GO") | .url'

# Why was an issue skipped?
gh claimcheck owner/repo#42 --json | jq -r '.[0].findings[] | select(.level == "block") | .title'
```

**Default repository.** Inside a clone, bare numbers (`123`, `#123`) use that repository. Use `-R owner/repo` or the `GH_REPO` environment variable to choose another.

**Colour** follows the same rules as `gh`: off when output is piped, and controlled by `NO_COLOR`, `CLICOLOR` and `CLICOLOR_FORCE`.

## How it works

- **Read-only.** It never comments, assigns, forks or opens anything. It uses the token from your `gh` login, so it sees the same repositories you can.
- **Light on rate limits.** One GraphQL query per issue gets the issue, comments, linked PRs, cross-references, community files and recent merges. One or two searches look for similar PR titles. Checks run three at a time.
- **Heuristics, not certainty.** Claim and pushback detection read plain English comments. A **CAUTION** means "go and look", not "don't". When in doubt, ask on the issue; maintainers almost always prefer that.

## FAQ

**Is it the same as the "linked pull requests" panel on the issue?**
No. That panel only shows PRs that use a closing keyword. claimcheck also finds PRs that mention the issue, PRs with a similar title that never mention it, PRs in the repository the project moved to, and claims that only exist as comments.

**Can I use it on private repositories?**
Yes, any repository your `gh` login can read.

**Does it work with GitHub Enterprise?**
It uses your `gh` host settings, so set `GH_HOST` for an Enterprise server.

**It flagged a PR that's not related.**
Title matching is fuzzy. Only close matches are warnings; the rest are grouped in one "share keywords" line. Please open an issue with the example so the matching can improve.

## Contributing

Issues and pull requests are welcome. The checks are plain functions over plain data, so most changes need no network:

```sh
go test ./...
```

- `internal/check`: the rules and verdicts (pure; start here)
- `internal/text`: comment and README parsing (claims, pushback, AI policy, moved notices, title similarity)
- `internal/github`: the only package that talks to the API
- `internal/render`: terminal output

Please run `gh claimcheck` on the issue first. It's only fair.

## License

[MIT](LICENSE)

<!-- Keywords: gh extension, GitHub CLI extension, gh cli plugin, open source contribution, first contribution, first-timers, good first issue finder, help wanted, hacktoberfest, check issue before claiming, avoid duplicate pull requests, duplicate PR detection, issue triage, is this issue taken, contributor tool, beginner friendly open source, developer tools -->
