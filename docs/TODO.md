# TODO

**English** | [Türkçe](tr/TODO.md)

Last updated: 2026-10-14. This is a feature roadmap only. Completed work and verification history:
[PROGRESS.md](PROGRESS.md). Security review and hardening: [SECURITY.md](SECURITY.md). Manual acceptance checklist:
[TESTING.md](TESTING.md).

## Planned features

- More UI languages beyond the current seven (English, Turkish, Arabic, German, Spanish, Italian, Russian — added
  2026-10-14, see `web/src/i18n/`), and whether the fallback for an unmatched browser language should stay Turkish
  (`FALLBACK` in `web/src/i18n/index.svelte.ts`).
- README polish for a public announcement: a release badge, a CI badge, a "why mumweb?" comparison with other
  Mumble web clients (the screenshot and demo GIF are done, see `docs/images/`).
- Invite link (signed, expiring token) as an alternative or addition to the access password.
- Editing your own comment; showing/uploading avatars.
- Viewing the channel description (editing already exists).
- Whisper/shout (VoiceTarget).
- Importing `.p12` certificates from the desktop client (PEM only today).
- Browser notifications (messages/joins while the tab is in the background), unread count in the tab title.
- Screen Wake Lock on mobile, to reduce audio cut-outs when the screen turns off.
- Fix the Android background drop: the session leaves the room 15-20 s after the browser goes to the background (a
  locked screen is fine). Find out whether the tab is frozen or the socket is killed, then shorten the ping while
  hidden and/or reconnect fast on `visibilitychange`.
- Firefox for Android reports "codec not supported" on login; find out which of `AudioEncoder`/`AudioDecoder` or the
  Opus configuration is missing there and show a precise message instead.
- Pasting images into chat (Mumble `data:` URI images, `ImageMessageLength` limit).
- Advanced audio settings: bitrate, jitter buffer, noise suppression options; per-listener volume for channel
  listeners (`listening_volume_adjustment`).
- Admin panel: move users between channels, a persistent audit log file, mapping an admin Mumble group to panel
  access instead of one shared password.
- Admin panel: make ban-list writes diff-based (add/remove by key against a freshly fetched list) instead of
  replacing the whole list, so a stale panel tab can't undo a ban made elsewhere.
- Capacity: a load-test tool in the repo, a VPS-size-to-`MUMWEB_MAX_SESSIONS` table, and a clear "server full"
  message in the UI instead of a reconnect loop (report capacity through `/config` / `/healthz`).

## Known limitations (not planned to change)

- Banning a web user from the desktop client also bans the gateway's IP, i.e. every web user. The web UI's own
  "Ban…" avoids this by banning only the certificate. No general mitigation planned (small groups); a "guard
  account" could automate unbanning the gateway IP if this becomes a real problem.
- A global push-to-talk key while the browser tab isn't focused is impossible (browser limitation); use voice
  activation or the desktop client while gaming.
- Mumble rate-limits messages per user (1/s, burst 5) and silently drops the rest, same as the desktop client.
- `SuperUser` cannot speak in Mumble, by design.
- No WASM Opus fallback for old Safari/iOS; not planned.
