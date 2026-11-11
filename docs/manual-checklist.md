# Manual checklist

Hardware cannot be unit-tested. Run these on a real laptop before trusting the recorder with a meeting, and write the result and the numbers in the table. The first eight come word for word from the design (§9.4); the rest come from the build.

Build the app first:

```sh
CGO_ENABLED=0 go build -trimpath -ldflags=-H=windowsgui -o voice-recorder.exe ./services/recorder/cmd/voice-recorder
```

To watch memory, start it from a terminal with `DEBUG_ADDR=127.0.0.1:6060` set, then run `go run ./tools/agentctl perf capture --url http://127.0.0.1:6060` and read the heap profile. Task Manager's "Memory (private working set)" column is the number to write down.

| # | Check | How | Expected | Result (date, machine, numbers) |
| --- | --- | --- | --- | --- |
| 1 | A 2-hour recording | Record a real or simulated 2-hour meeting. Note the file size and the memory at 5 minutes and at 2 hours. | 10–15 MB per hour; memory about 30 MB and flat. | 2026-11-11, ASUS desktop + Logi C270: memory 15 MB while recording (owner); the 2-hour run is still to do |
| 2 | Pause and resume | Pause for a minute, resume, stop. | One file; the timer leaves out the paused minute; the file plays straight through. | 2026-11-11, ASUS desktop + Logi C270: pass (owner) |
| 3 | Close the browser and reopen it | Mid-recording, close every browser window, wait a minute, open http://127.0.0.1:47811. | Recording carried on; the page shows the live timer and meter. | 2026-11-11, ASUS desktop: pass (owner) |
| 4 | Plug in a USB mic, switch to it, unplug it | Mid-recording: plug in a USB mic, read the status line, choose it, talk, unplug it. | Plugging in does not switch; choosing switches within about a second; unplugging falls back to the built-in mic; one file. | |
| 5 | Kill the process | Mid-recording, end `voice-recorder.exe` in Task Manager. Open the file in VLC. | The file plays up to the last second or so before the kill. | |
| 6 | Launch the app twice | Double-click the exe while it runs. | The running copy's page opens; there is still one process. | 2026-11-10, ASUS desktop: pass (log: "already running, opening its page") |
| 7 | Quit while recording | Press Quit mid-recording and confirm. | The file is finalised; the page says the app has quit. | |
| 8 | Playback | Open a recording in VLC and in Windows Media Player. | Both play it, with the right length. | |
| 9 | Noise cleanup on real meetings | Record 5 minutes with cleanup on, then 5 with it off, in the same room, once in Japanese and once in English. Transcribe each with faster-whisper. | Cleanup on is no worse than off; if it is, switch the default (ADR-0018). | |
| 10 | A mic that is not 48 kHz mono | Record with a Bluetooth headset (16 kHz) or a 44.1 kHz USB mic. | Normal speed and pitch; the length matches the clock. | 2026-11-10, Logi C270 (48 kHz stereo): delivered as 48 kHz mono, pass |
| 11 | The folder dialog | Press Browse, pick a folder, then press Browse and cancel. | The dialog opens in front of the browser; the picked folder is saved; Cancel changes nothing. | 2026-11-11, ASUS desktop: opens in front, Cancel works (owner) |
| 12 | Only this machine can reach the page | From a phone on the same network, open http://(the laptop's IP):47811. | It does not connect. | |
| 13 | The port is free | Run `netsh interface ipv4 show excludedportrange protocol=tcp`. | 47811 is in no excluded range. | 2026-11-10, ASUS desktop: free |
| 14 | Nothing to install | Run `objdump -p voice-recorder.exe` or copy the exe to a machine with no Go and no compiler. | Only Windows system DLLs; it starts. | 2026-11-10: imports only kernel32.dll statically |
| 15 | Antivirus | Copy the exe to the target laptop and run it. | Not quarantined. A build that linked miniaudio was (ADR-0020). | 2026-11-10, ASUS desktop with Bitdefender and Kaspersky: not quarantined |
| 16 | Closing the lid | With the tip in mind, close the lid mid-recording, reopen it a minute later. | Follows Windows' power settings; if the laptop slept, the recording carries on in the same file after waking. | |
| 17 | CPU | Watch Task Manager during a recording. | A few percent of one core (the spike measured about 10% of one i3-12100F core). If it matters, profile before changing anything (`perf-diagnose`). | |
