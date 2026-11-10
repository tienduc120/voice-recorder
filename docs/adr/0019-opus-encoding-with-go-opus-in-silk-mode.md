# 0019 - Opus encoding with go-opus in SILK mode

- **Status**: accepted
- **Date**: 2026-11-10

## Context

Recordings must be small (spec: 10–15 MB per hour) and clear enough for speech-to-text, which matters more than fidelity; meetings are in Japanese and English. The spec chose Opus in Ogg, mono, in speech (VOIP) mode at about 24 kbps, with 32 kbps if the encoder has only the CELT mode, and named pure-Go encoders such as `skrashevich/go-opus` and `kazzmir/opus-go` as first choice. No ffmpeg, and the shipped exe is pure Go. go-base runs every test with `-race`, which turns on the runtime's `checkptr` pointer checks.

Both candidates are libopus transpiled to Go. Measured in the spike and during this task:

- kazzmir/opus-go v1.7.0 (BSD-3-Clause) drives libopus's VOIP mode: 20.9 kbps on cleaned speech, 9.6–10.4 MB per hour. But its encoder aborts at creation under `-race` (`checkptr: pointer arithmetic result points to invalid allocation` in `silk_Get_Encoder_Size`), and its decoder trips a libopus assertion on valid packets. No gated test could use it.
- skrashevich/go-opus v0.1.1 (Apache-2.0, no dependencies) passes `-race`. Its general encoder forces CELT only; at the spec's CELT fallback of 32 kbps it measured 14.9 MB per hour. Its `EncodeSILK` encodes SILK, Opus's speech codec, from 16 kHz input; its low-level C-style API needs `uintptr` memory that `go vet`'s `unsafeptr` check rejects.
- faster-whisper `small`, on cleaned text-to-speech meetings with office noise at −35 and −25 dBFS (English WER, Japanese CER): opus-go VOIP 24 kbps 4.1/4.1 and 5.9/6.7; go-opus CELT 32 kbps 5.1/5.4 and 6.4/6.1; go-opus SILK wideband 24 kbps 4.1/4.1 and 5.9/7.5.

## Decision

1. Opus encoding uses `github.com/skrashevich/go-opus` v0.1.1 in SILK wideband mode (`EncodeSILK` with `BandwidthWideband`), mono, 24 kbps, VBR, complexity 10, 20 ms frames, behind `infrastructure/opusenc`.
2. The 48 kHz pipeline is decimated to 16 kHz by `dsp.Decimator`, a 63-tap Blackman-windowed sinc low-pass at 7 kHz that keeps one sample in three. Audio above 8 kHz is dropped; speech-to-text resamples to 16 kHz anyway.
3. The pre-skip written to OpusHead is the whole measured encode-decode delay at 48 kHz (decimator plus codec, 338 samples in the spike), so decoded audio lines up with what the mic heard; a test pins it by cross-correlation.
4. Tests decode with go-opus too, in `opusenc/opustest`. opus-go is not used.
5. go-opus frees its codec memory through `runtime.AddCleanup`, so the adapter's `Close` has nothing to release.

## Consequences

- Positive: the exe stays pure Go; every encoder path runs under the race detector; one library for encoding and decoding; about 10.8 MB per hour at 1.3% of one core; transcripts as good as libopus's VOIP mode within the test's resolution.
- Negative: the recorder owns a decimator; recordings sound like wideband telephone audio to a person, with nothing above 8 kHz. A bug in go-opus's transpilation would reach every file; the round-trip tests and the manual playback check guard against it.
- Neutral: if a later go-opus or opus-go release fixes the gaps (a VOIP encoder API, `checkptr`-clean code), switching is a change inside `opusenc` only.

## Alternatives considered

- **opus-go VOIP**: the best numbers, but untestable under the gate's `-race`; disabling `checkptr` for it would need a gate change.
- **go-opus CELT at 32 kbps** (the spec's fallback): simplest, but larger files and about a point worse English accuracy.
- **go-opus's low-level libopus API at 48 kHz**: full VOIP mode, but raw C memory through `uintptr` that `go vet` rejects.
- **libopus through cgo**: the reference, but brings cgo back into the product.
