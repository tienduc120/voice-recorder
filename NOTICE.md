# Third-party material

Only MIT, BSD, and Apache-2.0 material is brought in. For each item keep the upstream license text in the repository, record the upstream and version below, and prefer a dependency to a copy. A new dependency needs an ADR.

## Vendored or adapted

| Item                        | Upstream                                         | Version                                               | License                                    | Where                                                                             |
| --------------------------- | ------------------------------------------------ | ----------------------------------------------------- | ------------------------------------------ | --------------------------------------------------------------------------------- |
| golangci-lint configuration | https://github.com/maratori/golangci-lint-config | main as of 2026-10-17 (targets golangci-lint v2.14.0) | MIT, Copyright (c) 2021-2025 Marat Reimers | `.golangci.yml`; license text in `LICENSES/maratori-golangci-lint-config.txt` |

## Go dependencies (used as modules, not copied)

- `golang.org/x/tools` (BSD-3-Clause), the analysis framework behind our analyzers.
- `github.com/google/pprof` (Apache-2.0), the profile parser and diff behind `agentctl perf`.
- `go.uber.org/goleak` (MIT, Copyright (c) 2018 Uber Technologies, Inc.), the goroutine-leak check in `TestMain`.
- `github.com/VoiceBlender/rnnoise-go` v0.3.0 (BSD-3-Clause; Copyright (c) 2007-2017, 2024 Jean-Marc Valin, (c) 2023 Amazon, (c) 2017 Mozilla, (c) 2005-2017 Xiph.Org Foundation, (c) 2003-2004 Mark Borgerding), the noise cleanup, a port of Xiph's RNNoise with its trained weights, and its dependency `golang.org/x/sys` (BSD-3-Clause, The Go Authors). Linked into the shipped exe, so these notices travel with it.

`golang.org/x/tools` and `github.com/google/pprof` are dependencies of the `tools` module only. `goleak` and the audio libraries belong to the root module, which holds the platform and the recorder.

## Developer tools (installed, not copied)

golangci-lint, gofumpt, govulncheck, lefthook, arch-go. Each keeps its own license; they are checked by `go run ./tools/agentctl doctor`.
