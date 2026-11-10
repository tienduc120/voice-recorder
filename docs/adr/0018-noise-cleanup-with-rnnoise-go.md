# 0018 - Noise cleanup with rnnoise-go

- **Status**: accepted
- **Date**: 2026-11-10

## Context

The recorder has one noise-cleanup switch that must remove non-voice noise in an open-plan office (air-con, fans, typing, chairs) and keep every voice, including nearby colleagues. The spec names Xiph's RNNoise, a small neural speech denoiser on 10 ms frames at 48 kHz, with a pure-Go port as first choice and the C source through cgo as fallback. It also warns that speech-to-text can do worse on over-cleaned audio, so cleanup must be judged by transcripts, not by ear: "when unsure, clean less". Meetings are in Japanese and English.

The spike (Task 1) measured `github.com/VoiceBlender/rnnoise-go` v0.3.0 on this machine: 1.6% of one core at 48 kHz, noise in speech pauses down from −39 to −70 dBFS, speech level kept within 0.4 dB. faster-whisper `small` on synthetic Japanese and English meetings with office noise, decoded from the Opus files the recorder writes, gave (English WER, Japanese CER, noise at −35 / −25 dBFS): raw 7.4/4.1 and 5.9/5.9; uncapped cleanup 4.1/5.4 and 5.1/7.5; cleanup with a −20 dB gain floor 4.1/4.1 and 5.9/6.7. Uncapped cleanup hurt on heavy noise in both languages; the floor never cost much.

## Decision

1. Noise cleanup uses `github.com/VoiceBlender/rnnoise-go` v0.3.0 (BSD-3-Clause, a port of Xiph's RNNoise; the trained weights ship under the same licence in its `model` package).
2. It runs at 48 kHz on 480-sample frames, with samples scaled to the 16-bit range the model was trained on, behind `infrastructure/denoise`, so no other package sees the library.
3. `GainFloorDB` is −20: no band is attenuated by more than 20 dB, which keeps quiet and distant speech intact at the cost of some residual noise.
4. Cleanup is on by default. A real-meeting transcript comparison in both languages is on the manual checklist; if it shows cleanup hurting, the default changes, not the floor alone.

## Consequences

- Positive: pure Go, so the shipped exe stays free of cgo; bit-exact with upstream C at 48 kHz; allocation-free after construction; small CPU cost.
- Negative: the 3.4 MB of weights are linked into the exe. The output lags the input by 20 ms (960 samples), which nothing downstream needs to correct. The synthetic transcript test used text-to-speech voices; real voices may differ.
- Neutral: the owner can switch cleanup off per recording; the high-pass filter and the leveller run either way.

## Alternatives considered

- **Xiph RNNoise C source through cgo**: the same model, but cgo builds were already a problem on the owner's machine (antivirus) and the port is exact.
- **No floor (upstream default)**: cleaner pauses, but worse transcripts on heavy noise in both languages.
- **A −12 dB floor**: no better than −20 dB in the test, with more residual noise.
- **Cleanup off by default**: better only for Japanese at heavy noise, and by less than a point.
