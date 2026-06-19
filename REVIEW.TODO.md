# JamCapture — Architecture Review Backlog

Findings from the architecture audit (2026-06-18). No code changed yet.
Ordered by leverage, highest first.

> Key framing: no audio sample flows through Go — FFmpeg/JACK does all real-time
> work, Go only orchestrates. Go's GC is **not** in the audio path, so don't tune
> GC for audio; focus on the orchestration concurrency and on surfacing xruns.

## P1 — Concurrency data races (run `go test -race ./...` to confirm)
- [ ] `server.cfg` mutated without a lock while HTTP handlers read it
      (`server.go` ~432, ~842). `profileLock` does NOT guard `cfg`.
- [ ] `service.LoadProfile` reassigns `s.cfg` / `s.recorder` unsynchronized
      (`service.go:350-351`).
- [ ] `GetChannelStatus` writes `channelStatusCache` under `RLock`
      (`pipewire_recorder.go:283` → `:329`).
- [ ] `stdoutBuf` / `stderrBuf` written by `readOutput` goroutines, read in
      `stopFFmpeg`, no synchronization.
- [ ] `Cleanup()` touches `ffmpegCmd` without the mutex.

## P1 — `pw-link` fork-storm
- [ ] `ValidatePort` → `ListPorts` forks a full `pw-link -io` per source; the
      500ms source monitor runs checkAllSources **then** hasDuplicates each tick
      → dozens of subprocesses/sec. Mutualize: one `pw-dump` / `pw-link -io`
      per tick, parsed once and shared.

## P2 — Audio reliability / observability
- [ ] Xruns are detected but never surfaced (`pipewire_recorder.go:464` logs
      only). Add to `RecordingSession` + web status.
- [ ] Ensure the FFmpeg JACK client gets RT scheduling priority
      (rtkit / `RLIMIT_RTPRIO`) — currently not guaranteed.
- [ ] No FFmpeg health-check during recording — the worker just waits
      `<-stopChan` (`:203`); a dead FFmpeg is noticed only at Stop().

## P3 — Maintainability
- [ ] Unify duplicated `globalIdx` → `jamcapture_rec:input_N` logic
      (`recordingWorker` `:168` vs `buildConnectionMap` `:726`).
- [ ] Split `server.go` (3060 lines, ~40 handlers) by domain.
- [ ] Bound/rotate FFmpeg stdout/stderr buffers (unbounded per-session growth).
