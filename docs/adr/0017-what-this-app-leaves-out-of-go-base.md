# 0017 - What this app leaves out of go-base

- **Status**: accepted
- **Date**: 2026-11-10

## Context

This repository starts as a copy of go-base, whose rules, gates, skills and platform packages the owner wants followed wherever they fit. go-base is built for cloud microservices backed by Postgres. This app is a single-user Windows desktop recorder with no database, no load to generate, and one bounded context whose shape (a local control page, a frame loop, hardware adapters) is not the Postgres-backed HTTP API that go-base's generator writes. The owner also ruled that no gate may be loosened to make the app fit: where a rule fights a real need, the owner decides.

## Decision

1. The database layer is left out: `platform/postgres`, sqlc and its query files, the `agentctl db check` and `db test` commands, the `db check` step of `verify`, the Postgres CI job, `tools/internal/dbcheck`, and the `writing-queries` skill. ADRs 0011 and 0012 stay as inherited records with nothing to apply to.
2. The service generator is left out: `agentctl new service`, `tools/internal/scaffold`, its witness `services/example`, and the `new-service` skill. ADR-0010 stays as an inherited record. `services/recorder` is laid out by hand to the same four layers, so `arch-go.yml` applies unchanged.
3. The load generator is left out: `platform/loadgen` and ADR-0013's load programs. Memory is checked by profiling a real recording (`DEBUG_ADDR` plus `agentctl perf capture`) instead.
4. `docs/deferred.md` is left out: it is go-base's roadmap (worker template, buf contracts, deploy gates), and the owner asked for no expansion plan.
5. `agentctl doctor` drops docker, sqlc and buf, which only the left-out parts need, and checks for a C compiler, which `go test -race` needs on Windows.
6. The recording session is not marked `//agent:aggregate`. It is never stored or restored, so ADR-0016's `Restore` would be dead code; the analyzer still runs and would judge any marked type.
7. Every other rule, gate, fixture, analyzer, hook, skill and inherited ADR stays as go-base has it. A CI job on Windows runs the fast gate, because the Linux job cannot see the Windows-only adapters.

## Consequences

- Positive: no tool demands Docker, Postgres or sqlc on a machine that never needs them, and every gate that remains still has a fixture that proves it fires.
- Negative: a change in go-base's database or generator parts never reaches this repository; that is intended. Updates to the parts that are kept must be copied by hand.
- Neutral: `txboundary` and `aggregateshape` still run in `verify`; with no transactions and no marked aggregates they report nothing until one appears.

## Alternatives considered

- **Keep everything from go-base**: every gate would still pass, but `doctor` would demand Docker and sqlc, CI would run a Postgres job against nothing, and readers would look for a database that is not there.
- **Generate `services/recorder` with `agentctl new service` and delete the Postgres parts**: the generator writes an HTTP API around a repository and a Postgres adapter; deleting most of it leaves less than a hand-written layout and hides why the shape differs.
- **Loosen a gate where the app needs it** (for example `gosec` on a Win32 call): refused by the owner; such a case goes to the owner first.
