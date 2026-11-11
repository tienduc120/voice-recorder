# Voice Recorder

Records meetings on a Windows laptop into small audio files (about 11 MB per hour) that a speech-to-text tool can read later. It keeps recording if you close the browser, unplug a mic, or the app crashes.

## Get the app

You need Windows 10 or 11.

1. Install [Go](https://go.dev/dl/) and [Git](https://git-scm.com/download/win).
2. Open PowerShell and run:

```powershell
git clone https://github.com/tienduc120/voice-recorder.git
cd voice-recorder
$env:CGO_ENABLED = "0"
go build -trimpath -ldflags=-H=windowsgui -o voice-recorder.exe ./services/recorder/cmd/voice-recorder
```

3. That makes `voice-recorder.exe`. Copy it anywhere, even to another laptop: it needs nothing else installed.

## Record a meeting

1. Double-click `voice-recorder.exe`. The recorder opens in your browser.
2. Pick a microphone and, if you like, type a title. Without one, the file is named after the date and time.
3. Press **Record** (or Space). Use **Pause** and **Stop** as needed.
4. Press **Open** to see your recordings. They go to `Documents\Recordings` unless you **Browse** to another folder.

Good to know:

- Closing the browser does not stop a recording. Double-click the exe again to get the page back.
- Plugging in a mic does not switch to it; choose it in the list. If the mic in use is unplugged, recording moves to Windows' default mic, in the same file.
- **Noise cleanup** (on by default) quiets steady background noise such as fans and air conditioning.
- Keep the laptop lid open: closing it may put the laptop to sleep.
- The app keeps running in the background, using about 15 MB of memory, until you press **×** at the top of the page.
- Settings and the log are in `%APPDATA%\VoiceRecorder`.

## Develop

Read [AGENTS.md](AGENTS.md) first. `go run ./tools/agentctl doctor` checks your tools and `go run ./tools/agentctl verify` runs every check. Before trusting a new build with a real meeting, go through [docs/manual-checklist.md](docs/manual-checklist.md).

## License

MIT, Copyright (c) 2026 Luu Tien Duc. See [LICENSE](LICENSE).

## Credits

- [go-base](https://github.com/tienduc120/go-base) by Luu Tien Duc (MIT): the layout, checks and tools this code base is built on.
- Audio: [go-wca](https://github.com/moutend/go-wca) and [go-ole](https://github.com/go-ole/go-ole) (MIT) capture the mic; [go-opus](https://github.com/skrashevich/go-opus) (Apache-2.0), the [Opus](https://opus-codec.org/) codec in Go, encodes it; [rnnoise-go](https://github.com/VoiceBlender/rnnoise-go) (BSD-3-Clause), a port of Xiph's [RNNoise](https://gitlab.xiph.org/xiph/rnnoise), cleans the noise; Robert Bristow-Johnson's [Audio EQ Cookbook](https://www.w3.org/TR/audio-eq-cookbook/) gives the high-pass filter; [RFC 3533](https://www.rfc-editor.org/rfc/rfc3533) and [RFC 7845](https://www.rfc-editor.org/rfc/rfc7845) define the file format.
- Tools: Go and [golang.org/x/tools](https://pkg.go.dev/golang.org/x/tools), [golangci-lint](https://github.com/golangci/golangci-lint) with a config adapted from [maratori's](https://github.com/maratori/golangci-lint-config) (MIT, [license](LICENSES/maratori-golangci-lint-config.txt)), [gofumpt](https://github.com/mvdan/gofumpt), [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck), [lefthook](https://github.com/evilmartians/lefthook), [arch-go](https://github.com/arch-go/arch-go), [goleak](https://github.com/uber-go/goleak) and [google/pprof](https://github.com/google/pprof).
- Ideas from [cc-skills-golang](https://github.com/samber/cc-skills-golang), Three Dots Labs' [wild-workouts-go-ddd-example](https://github.com/ThreeDotsLabs/wild-workouts-go-ddd-example) and [watermill](https://github.com/ThreeDotsLabs/watermill), [go-clean-template](https://github.com/evrone/go-clean-template), [pprof-analyzer-mcp](https://github.com/ZephyrDeng/pprof-analyzer-mcp), [yet-another-agent-harness](https://github.com/dirien/yet-another-agent-harness), [agents-md-cookbook](https://github.com/Taiizor/agents-md-cookbook), [go-arch-lint](https://github.com/fe3dback/go-arch-lint), [AGENTS.md](https://agents.md) and Factory's [Using linters to direct agents](https://www.factory.ai/using-linters-to-direct-agents).

What was copied or adapted, with its license: [NOTICE.md](NOTICE.md).
