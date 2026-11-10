# voice-recorder

A Windows meeting recorder: one `.exe` with a local control page that writes small, crash-safe Ogg/Opus speech files for later speech-to-text. It is built on go-base, a Go base for strict DDD and clean architecture extended by AI agents, and keeps go-base's rules a tool checks, skills an agent follows, and feedback loops for performance and tech debt. The design is in the spec the owner approved; the code base follows go-base wherever it fits.

The decisions behind it: [docs/adr/](docs/adr/README.md). ADRs 0001–0016 are inherited from go-base; the ones after are this app's. Add one with the `writing-adrs` skill.

## Status

Enforced today, inherited from go-base: the lint rules in `.golangci.yml`, the layer rules in `arch-go.yml`, and the analyzers in `tools/analyzers`, each proven by a fixture, plus Markdown style (one paragraph per line), the agent guard (below), the pprof diagnosis loop (`agentctl perf`, ADR-0006), goroutine-leak checks in tests (ADR-0007), the technical-debt loop (`agentctl debt`, ADR-0008, whose ratchet `verify` runs), and the ADR check. What this app leaves out of go-base (the database layer, the service generator and its witness, the load generator) is recorded in its own ADR. Do not report a rule as enforced unless a tool in this repo checks it.

## How to work

- **Test first, and run the tests.** Write the failing test, run that package's tests and see it fail for the right reason, make it pass, run them again. That is the only checking you do while working.
- **Do not run lint, format, `verify`, `rules test`, or a review between steps.** The pre-commit hook runs `verify` and CI runs `verify --full`; a rejected commit tells you exactly what to fix. Run `verify` by hand only to read the details of a failure the hook already reported.

## Commands

Run from the repository root. There is no Make; `agentctl` is the command surface.

| Need                                                              | Command                                                                                        |
| ----------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Run the gates (hooks and CI do this; by hand only to investigate) | `go run ./tools/agentctl verify` (about 12s); `--full` adds govulncheck and `rules test` |
| Check a commit message                                            | `go run ./tools/agentctl commitmsg <file>` (the commit-msg hook calls it)                    |
| Install the git hooks (once per clone)                            | `lefthook install`                                                                           |
| Check the toolchain and pinned tools                              | `go run ./tools/agentctl doctor` (`--json` for machines)                                   |
| Prove every rule fires on its fixture                             | `go run ./tools/agentctl rules test`                                                         |
| Find what owns the time, memory, locks, or goroutines | `go run ./tools/agentctl perf capture --url http://127.0.0.1:<pprof port>` while the app records; `perf analyze [--base before.pprof] <file>` for one profile (skill: `perf-diagnose`) |
| Find, triage, and pay off the debt the gates allow | `go run ./tools/agentctl debt scan`, then `debt confirm <id>`, `debt next`, `debt check` (skill: `debt-recovery`) |
| Record a decision | `go run ./tools/agentctl adr new "<title>"` writes the next record and indexes it; `adr check` validates them all (skill: `writing-adrs`) |
| Make the skills visible to Claude Code (once per clone) | `go run ./tools/agentctl skills link`; `skills check` fails if the link is missing |
| Run anything that could hang                                      | `go run ./tools/agentctl run -- <command> [args...]` (`--timeout`, `--idle`, `--name`) |
| Tests (the TDD loop)                                              | `go test -race -timeout 120s -count=1 ./services/recorder/<package>/` or `./tools/<package>/`  |
| Format Go (apply)                                                 | `cd tools && golangci-lint fmt ./...`                                                        |
| Format Markdown (apply)                                           | `go run ./tools/agentctl fmt md [file...]`                                                   |
| Layer rules on real code                                          | `arch-go` (reads `arch-go.yml` from the module root)                                       |
| Build the app | `CGO_ENABLED=0 go build -trimpath -ldflags=-H=windowsgui -o voice-recorder.exe ./services/recorder/cmd/voice-recorder` |
| Profile memory while recording | start the app with `DEBUG_ADDR=127.0.0.1:6060`, then `go run ./tools/agentctl perf capture --url http://127.0.0.1:6060` (skill: `perf-diagnose`) |

Two modules joined by `go.work`: the root module (services, platform, contracts) and `tools/` (agentctl). Tool dependencies must never enter the root module.

## Hard rules

- **Test first.** No production code without a test that failed first, for the right reason. Follow the `writing-go-tests` skill for layout and what each layer tests with.
- **Windows-only code lives in `infrastructure` or `cmd` files ending `_windows.go`**, and each such package keeps a portable `doc.go`, so the Linux CI job still builds and checks everything else.
- **Open a mic by its explicit WASAPI endpoint ID**, never through Windows' default-device routing: such a stream follows the default, and plugging in a USB mic can change the default mid-meeting.
- **Start goroutines only through `platform/async`** (`async.Run`): it cancels, waits, and turns a panic into an error. A service serves pprof on a loopback-only port with `platform/debugserver`, never on the serving port (ADR-0009).
- **A package that starts goroutines, subprocesses, or servers checks for leaks.** Its `TestMain` calls `goleak.VerifyTestMain(m)` (ADR-0007), so a test that leaves something running fails the package. Never `time.Sleep` in a test.
- **New debt is triaged, not hidden.** A `//nolint`, a TODO with no reference, or a function near the complexity limit is debt: fix the cause, or ask the user to accept it with a reason and an expiry. A `refactor:` commit may only add tests, never edit one (the commit-msg hook enforces it). Follow the `debt-recovery` skill.
- **Never optimise without a profile.** Follow the `perf-diagnose` skill in `.agents/skills/`: capture, read the findings, make one change, capture again, and quote the numbers before and after.
- **Evidence, not claims.** "Tests pass" means you ran them and saw the result. Name every step you skipped and why.
- **Never weaken a gate to make a change pass.** `.golangci.yml`, `arch-go.yml`, and everything under `testdata/` are the rules that judge you: change them only with the user's explicit approval. Fix the cause instead.
- **Every rule has a fixture.** A new or changed rule gets a violation fixture in `testdata/violations/<id>/` (with a `rule.json` naming the enforcer and the text it must report) and `rules test` must stay green. The compliant twin in `testdata/clean/` must stay free of findings.
- **Never bypass git hooks** (`--no-verify`). `lefthook.yml` runs `verify` before each commit, `commitmsg` on the message, and `verify --full` before a push.
- **Anything that can run long goes through `agentctl run`.** It enforces a deadline and an idle limit, kills the whole process tree, and keeps the full output in `.agentctl/logs` so only a short summary comes back (exit 124 means it was killed). It also gives `go test` its own `-timeout`, so a hang ends with a goroutine dump showing where it is stuck. Servers never run in the foreground of an agent session.
- **Skills live in `.agents/skills/<name>/SKILL.md`**, readable by any agent. `.claude/skills` is a git-ignored link to that directory, made by `agentctl skills link` (a junction on Windows, no administrator rights needed); never put a skill there directly.
- **Do not commit `.local/`** (gitignored scratch and notes) and do not install software system-wide without asking.
- **Credit what you reference.** When a design, config, or code comes from a public source, add it to the README "Credits" in the same change. If any text or code is copied or adapted, also keep the upstream license text and record upstream and version in `NOTICE.md`. Only MIT, BSD, and Apache-2.0 material comes in.
- **Markdown: one paragraph or list item per line.** Never hard-wrap prose; editors wrap for themselves and hand-wrapped lines show as arbitrary breaks. `agentctl fmt md` fixes it and `verify` checks it.
- **Errors are returned values, owned by the layer that raises them** (`(T, error)`, never a `Result[T]` or a panic for an expected failure). Translate at each layer boundary with `%w`, and let only `interfaces` choose a status. Follow the `handling-errors` skill (ADR-0014).
- **Comments say why**, never what, and stay short.
- Conventional Commits: `type(scope): subject`, imperative, lowercase.

## Agent guard

In Claude Code, `.claude/settings.json` runs `agentctl hook` before every edit and shell command and refuses, with a reason you can act on:

- changing a committed rule file: `.golangci.yml`, `arch-go.yml`, `lefthook.yml`, the CI workflow, `.claude/settings.json`, the gate code (`tools/analyzers`, `tools/agentlint`, `tools/internal/{rules,verify,commitmsg,guard,adr}`), anything under `testdata/`, and the licenses. New files are always allowed, so adding a fixture or an analyzer is fine; protection starts once a file is committed.
- skipping the git hooks: `--no-verify`, `HUSKY=0`, `LEFTHOOK=0`, `core.hooksPath`, `lefthook uninstall`.
- writing, moving, or deleting a protected path through the shell.
- commands that never return: `go test` without `-timeout`, a service run in the foreground, `docker compose up` without `-d`, `tail -f`. Use `agentctl run` instead.

When you are blocked, fix the cause. Do not look for another way to make the same change. If the rule really should change, ask the user: they approve by starting the session with `AGENTCTL_ALLOW_PROTECTED=1`, which only lifts the protected-file block. The guard is a guard rail, not a security boundary; `CODEOWNERS`, the git hooks, and CI are the boundary. Codex and Cursor do not run these hooks. Decision record: [ADR-0005](docs/adr/0005-agent-guard.md).

## Architecture

One bounded context, `services/recorder/`, layered inside a compiler-private `internal/`: `domain`, `application`, `infrastructure`, `interfaces`; `cmd/<binary>` is the composition root.

Enforced by `arch-go.yml` (fixture per rule):

- `domain` imports only other domain packages and a small pure standard-library set: no `context`, no I/O, no frameworks.
- `application` imports domain, application, `platform`, `contracts`, and a restricted standard-library set. It declares the ports (repository and clock interfaces) that infrastructure implements.
- `infrastructure` imports domain, application, itself, `platform`, `contracts`; never `interfaces`.
- `interfaces` imports application, itself, `platform`, `contracts`; never domain or infrastructure.
- `platform` never imports a service or `contracts`; `contracts` never imports a service or `platform`.

Enforced by `.golangci.yml` (fixture per rule): no `context.Background`/`TODO` outside `main`; no `http.Get`/`DefaultClient`/`DefaultTransport`; no `time.Sleep`; no `fmt.Print*`; no `log` (use `slog`); no global logger; no mutable package state; HTTP requests carry a context; rows and response bodies are closed; no `SELECT *`; every `//nolint` names its linter and gives a reason.

Enforced by the analyzers in `tools/analyzers` (run as `go vet -vettool`, fixture per rule): inside the function passed to `InTransaction` only repositories, the outbox, and the clock are used, with no network call, no nested transaction, and no transaction opened outside `application` (`txboundary`, ADR-0015); a type marked `//agent:aggregate` has a typed `id`, `New<T>`, and `Restore<T>`, and only infrastructure calls `Restore` (`aggregateshape`, ADR-0016); no `go` statement outside `platform/async`; domain types have no exported fields (opt out per type with `//agent:allow-exported-fields <reason>`), no struct tags, and never call `time.Now`/`Since`/`Until`.

The Go compiler already forbids importing another service's `internal/` packages.

Planned, not enforced yet: nothing; this app has no database, outbox or transaction runner, so `txboundary` and `aggregateshape` have nothing to judge until one appears.

## Things that will surprise you

Grow this by correction: when any tool gets something wrong, add the line.

- **golangci-lint dedups per line by default**; here `uniq-by-line` is off, so one line can carry several findings. If a rule seems silent, check that it is not hidden behind another finding on the same line.
- **One fixture that does not compile silences every finding** from a linter, so `rules test` type-checks the staged module first and names the broken fixture.
- **Fixture import paths are relative to the staged module `fixture.test/<fixture-id>/...`**, not this repository's module path.
- **On Windows, Python's `write_text` writes CRLF** and `.gitattributes` forces LF; write bytes with explicit `\n`. Shell heredocs can also swallow double backslashes: write files that contain escapes with the editor tools.
- **`rm` with a variable or wildcard target is refused by the sandbox**; remove by literal path.
- **Never name a package `httpapi`.** On the maintainer's Windows machine Bitdefender quarantines a test binary called `httpapi.test.exe` (a heuristic detection, `Gen:Variant.Yogi`), and `go test` fails with `Access is denied`. The binary's name is the trigger, not its content; the delivery package is `interfaces/rest`. If a test build fails like this, rename the package rather than rebuilding in a loop.
- **Any build that links miniaudio (malgo) is quarantined at link time by the maintainer's antivirus** (Bitdefender and Kaspersky are both active): `go build` fails with `Access is denied` on its temporary `a.out.exe`. That is why capture is pure Go (go-wca); see the capture ADR.
- **go-wca's `IMMNotificationClient` crashes the process when Windows calls it.** Never register it; poll the device list instead.
- **opus-go's decoder trips a libopus assertion** (`silk/resampler.c`) on valid packets and aborts. Decode in tests with go-opus (`opusenc/opustest`); FFmpeg agrees with go-opus.
