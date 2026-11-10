# voice-recorder

Records in-person meetings on a Windows laptop's built-in or USB mic into small Ogg/Opus files (about 10 MB per hour) that a speech-to-text tool reads later. Noise cleanup is one switch. A recording survives a closed browser tab, a mic plugged in or out, and a crash, and the app uses about 30 MB of memory while it records.

Status: under construction. Read [AGENTS.md](AGENTS.md) first, then [docs/adr/](docs/adr/README.md).

## Quick start

You need Windows 10 or 11, Go (the version in `go.mod` is fetched automatically), Git, and a C compiler for the race-detector tests. The finished `.exe` is pure Go and needs nothing installed.

```sh
go run ./tools/agentctl doctor     # checks the toolchain and pinned tools, prints install hints
go run ./tools/agentctl verify     # runs every gate; --full adds govulncheck and the rules proof
lefthook install                   # run the gates on every commit and push
```

There is no Make. `agentctl` is the single command surface.

## License

MIT, Copyright (c) 2026 Luu Tien Duc. Free to use, modify, and use commercially. See [LICENSE](LICENSE).

## Credits

This app stands on public work. Material that is copied or adapted is listed in [NOTICE.md](NOTICE.md) with its license; this section thanks everyone whose work shaped a decision, whether or not any text was copied. A source moves from "referenced" to "used" the moment code or configuration from it lands here.

**Used**

- [go-base](https://github.com/tienduc120/go-base) by Luu Tien Duc (MIT): the layout, gates, analyzers, `agentctl`, skills and platform packages this code base is built on.
- The Go team and community: the language, toolchain, standard library, `go vet`, and [golang.org/x/tools](https://pkg.go.dev/golang.org/x/tools) (the analysis framework behind our custom analyzers).
- Developer tools we run, not copy: [golangci-lint](https://github.com/golangci/golangci-lint), [gofumpt](https://github.com/mvdan/gofumpt), [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck), [lefthook](https://github.com/evilmartians/lefthook) (Evil Martians), [arch-go](https://github.com/arch-go/arch-go).
- [goleak](https://github.com/uber-go/goleak) by Uber (MIT): the `TestMain` check that fails a package whose tests leave a goroutine running.
- [google/pprof](https://github.com/google/pprof) (Apache-2.0): the profile library behind `agentctl perf`. Our analyzer charges each sample to the nearest function in our own code and diffs snapshots on top of it.
- [golangci-lint-config](https://github.com/maratori/golangci-lint-config) by maratori (MIT): `.golangci.yml` is adapted from it, with the license text kept in [LICENSES/](LICENSES/maratori-golangci-lint-config.txt).
- Evaluated, not adopted: [go-arch-lint](https://github.com/fe3dback/go-arch-lint) by fe3dback. It informed the choice of arch-go; thank you.

**Referenced for design** (all MIT unless noted; no code copied yet)

| Source | By | What shaped us |
|---|---|---|
| [cc-skills-golang](https://github.com/samber/cc-skills-golang) | samber | the set of Go topics worth an agent skill |
| [wild-workouts-go-ddd-example](https://github.com/ThreeDotsLabs/wild-workouts-go-ddd-example) | Three Dots Labs | DDD, clean architecture, and CQRS layering in Go |
| [watermill](https://github.com/ThreeDotsLabs/watermill) | Three Dots Labs | broker-agnostic messaging behind a port |
| [go-clean-template](https://github.com/evrone/go-clean-template) | Evrone | service wiring, OpenTelemetry, and test layout |
| [pprof-analyzer-mcp](https://github.com/ZephyrDeng/pprof-analyzer-mcp) | ZephyrDeng | machine-readable pprof findings for agents |
| [yet-another-agent-harness](https://github.com/dirien/yet-another-agent-harness) | dirien | command guards and secret scanning as agent hooks |
| [agents-md-cookbook](https://github.com/Taiizor/agents-md-cookbook) | Taiizor | shape of a Go `AGENTS.md` |

Also: [AGENTS.md](https://agents.md) as the shared agent instruction format, and Factory's article [Using linters to direct agents](https://www.factory.ai/using-linters-to-direct-agents).
