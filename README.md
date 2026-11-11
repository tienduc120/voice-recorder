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

## Use

Build the app once; it needs no installer and nothing else on the target laptop:

```sh
CGO_ENABLED=0 go build -trimpath -ldflags=-H=windowsgui -o voice-recorder.exe ./services/recorder/cmd/voice-recorder
```

Double-click `voice-recorder.exe`. It opens its page in the default browser at http://127.0.0.1:47811; launching it again opens the same page. Recording carries on if the browser is closed. The app stops only through Quit on the page.

- Recordings go to `Documents\Recordings` unless you choose another folder, named `YYYY-MM-DD_HHMM_title.ogg` (Ogg Opus, mono, about 11 MB an hour).
- Settings are in `%APPDATA%\VoiceRecorder\config.json`, and the log in `%APPDATA%\VoiceRecorderoice-recorder.log` (the previous run's in `voice-recorder.log.1`).
- Space starts a recording, then pauses or resumes it, when no field has focus.

Before trusting it with a meeting, run [docs/manual-checklist.md](docs/manual-checklist.md) on the laptop.

## License

MIT, Copyright (c) 2026 Luu Tien Duc. Free to use, modify, and use commercially. See [LICENSE](LICENSE).

## Credits

This app stands on public work. Material that is copied or adapted is listed in [NOTICE.md](NOTICE.md) with its license; this section thanks everyone whose work shaped a decision, whether or not any text was copied. A source moves from "referenced" to "used" the moment code or configuration from it lands here.

**Used**

- [go-base](https://github.com/tienduc120/go-base) by Luu Tien Duc (MIT): the layout, gates, analyzers, `agentctl`, skills and platform packages this code base is built on.
- The Go team and community: the language, toolchain, standard library, `go vet`, and [golang.org/x/tools](https://pkg.go.dev/golang.org/x/tools) (the analysis framework behind our custom analyzers).
- Developer tools we run, not copy: [golangci-lint](https://github.com/golangci/golangci-lint), [gofumpt](https://github.com/mvdan/gofumpt), [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck), [lefthook](https://github.com/evilmartians/lefthook) (Evil Martians), [arch-go](https://github.com/arch-go/arch-go).
- [go-wca](https://github.com/moutend/go-wca) by Yoshiyuki Koyanagi and [go-ole](https://github.com/go-ole/go-ole) by Yasuhiro Matsumoto (both MIT): pure-Go WASAPI and COM, the mic capture (ADR-0020).
- [go-opus](https://github.com/skrashevich/go-opus) by Sergey Krashevich (Apache-2.0), the [Opus](https://opus-codec.org/) reference implementation transpiled to Go: the SILK speech encoder and the tests' decoder (ADR-0019).
- [rnnoise-go](https://github.com/VoiceBlender/rnnoise-go) (BSD-3-Clause), a pure-Go port of [RNNoise](https://gitlab.xiph.org/xiph/rnnoise) by Jean-Marc Valin and Xiph.Org: the noise cleanup (ADR-0018).
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
| [Audio EQ Cookbook](https://www.w3.org/TR/audio-eq-cookbook/) (W3C document licence) | Robert Bristow-Johnson | the high-pass filter's biquad coefficients |
| [RFC 3533](https://www.rfc-editor.org/rfc/rfc3533) and [RFC 7845](https://www.rfc-editor.org/rfc/rfc7845) (IETF Trust) | S. Pfeiffer; T. Terriberry, R. Lee, R. Giles | the Ogg page format and Ogg Opus mapping the muxer writes |

Also: [AGENTS.md](https://agents.md) as the shared agent instruction format, and Factory's article [Using linters to direct agents](https://www.factory.ai/using-linters-to-direct-agents).
