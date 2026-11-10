# 0020 - Audio capture with go-wca

- **Status**: accepted
- **Date**: 2026-11-10

## Context

The recorder captures the built-in or a USB mic on any Windows 10 or 11 laptop. It must open a mic by its ID so a newly plugged-in mic never takes over, notice when the mic in use goes away, and run for hours without growing memory. The spec named `github.com/gen2brain/malgo` (miniaudio through cgo) first and pure-Go WASAPI bindings as the fallback; malgo's licence was to be confirmed.

The spike (Task 1) found:

- Every build that links miniaudio was quarantined at link time by the owner's antivirus (Bitdefender and Kaspersky are both active): `go build` failed with `Access is denied` on its temporary `a.out.exe`. Builds of the same programs without malgo never were. A recorder that antivirus deletes fails "runs on any Windows laptop".
- malgo is Unlicense and miniaudio public domain or MIT-0, both outside the MIT, BSD and Apache-2.0 rule.
- `github.com/moutend/go-wca` v0.3.0 (MIT) and `github.com/go-ole/go-ole` (MIT) are pure Go. On a Logi C270 webcam (48 kHz stereo), shared mode with `AUTOCONVERTPCM | SRC_DEFAULT_QUALITY` delivered 48 kHz mono float as asked, about 1.5 s after opening. An unplug surfaced as `AUDCLNT_E_DEVICE_INVALIDATED` from `GetNextPacketSize` within about 2 s. The C270 also invalidated its endpoint on its own several times in 10 minutes while staying listed; reopening it worked each time. A 10-minute live recording held 31 MB of private memory, flat.
- go-wca is partly stubbed: `IMMDeviceEnumerator.GetDevice` takes no arguments, and its `IMMNotificationClient` crashed the process (exit 139) when Windows called it on an unplug. Its `PROPVARIANT.String` frees the string it reads, so freeing it again corrupts the heap (`0xC0000374`), which a first version of the adapter did.

The owner approved replacing malgo with go-wca on 2026-11-10.

## Decision

1. Capture uses `github.com/moutend/go-wca` v0.3.0 with `github.com/go-ole/go-ole` and `golang.org/x/sys/windows` (BSD-3-Clause, The Go Authors), behind `infrastructure/capture`, in files built only on Windows.
2. A stream opens one endpoint by its explicit WASAPI ID, found by enumerating active capture endpoints, in shared mode asking for IEEE float, 1 channel, 48 kHz with `EVENTCALLBACK | AUTOCONVERTPCM | SRC_DEFAULT_QUALITY`, so Windows converts from whatever the device delivers and never reroutes the stream to a new default.
3. All streaming runs on one goroutine locked to its OS thread, started through `platform/async` and initialised for COM; open and close requests are handed to it. Listing runs on the caller's thread with its own COM initialisation.
4. No device notifications are registered. The recorder polls the list every 2 s, and a stream reports its own loss when a read fails.
5. A `PROPVARIANT` read through go-wca's `String` is not freed again: `String` frees it.

## Consequences

- Positive: the shipped exe is pure Go and was not quarantined; the licences are MIT and BSD; device format conversion is Windows' own.
- Negative: go-wca is old and partly stubbed, so the adapter works around it, and a COM call it gets wrong can crash rather than return an error. Tests of real capture need a mic and skip on the CI runner, which has none.
- Neutral: a plug-in shows up within 2 s of polling rather than at once; the recorder never switches on a plug-in anyway.

## Alternatives considered

- **malgo (miniaudio)**: quarantined by antivirus and outside the licence rule.
- **An own WASAPI binding on `golang.org/x/sys/windows`**: no stubs, but several hundred lines of COM vtables to write and test; the step to take if go-wca blocks a fix.
- **go-wca's `IMMNotificationClient`**: instant plug events, but it crashed the process.
