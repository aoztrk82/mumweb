# Contributing

**English** | [Türkçe](CONTRIBUTING.tr.md)

Thanks for considering a contribution to mumweb.

## This project is AI-coded, and that's welcome here

mumweb's gateway, web client and documentation were written by [Claude Code](https://claude.com/claude-code)
working with the project owner, prompt by prompt, rather than typed by hand line by line. [CLAUDE.md](CLAUDE.md) —
the file that guides Claude in this repository — is the closest thing this project has to an architecture
document, and it is kept accurate for exactly that reason.

AI-assisted contributions are explicitly welcome, not just tolerated: a pull request written with Claude, Codex,
Copilot or any other assistant is judged the same way a hand-written one is — on whether it's correct, tested and
documented, not on how it was typed. You do not need to disclose that you used an assistant, but you're welcome to
mention it in the PR description if you'd like (for example, which model and what you asked it to do); either way
the review bar is the same. If you're bringing your own AI assistant to this repo, point it at `CLAUDE.md` first —
it already has the conventions, the layout and the protocol gotchas an assistant would otherwise have to rediscover.

## Before you start

- **Bug or small fix:** open a pull request directly.
- **New feature or a change in behaviour:** open an issue first, so we can agree on the approach before you (or
  your assistant) write the code. Check [docs/TODO.md](docs/TODO.md) — it may already be planned or explicitly
  marked as a known limitation that won't change.
- **Security issue:** do not open a public issue. Follow [SECURITY.md](SECURITY.md).

## Making a change

1. Fork the repository and create a branch for your change.
2. Follow the conventions in [CLAUDE.md](CLAUDE.md): Go code is `gofmt`-ed, Svelte 5 with runes and strict
   TypeScript, Conventional Commits (`feat:`, `fix(gateway):`, `docs:`, …), no new dependency without a reason.
3. Build and test locally — see [docs/BUILDING.md](docs/BUILDING.md) and [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).
   At minimum, before opening a PR:
   - `gofmt -l gateway` is empty (Windows checkouts are CRLF: `tr -d '\r' < f | gofmt -l`)
   - `go vet ./...` and `make test` (Go + Vitest) pass
   - `npm run check` (TypeScript) passes in `web/`
4. **Update the documentation in the same commit**, not as a follow-up — the table in CLAUDE.md's "Keep the
   documentation up to date" section says exactly which file to touch for which kind of change (README tables for a
   new env var or feature, `docs/PROGRESS.md` for anything verified or discovered, `docs/TODO.md` to tick or add
   items, and so on). Documentation changes need **both** languages: English first, then the Turkish mirror in
   `docs/tr/` (or `README.tr.md` / `CONTRIBUTING.tr.md` for the root files). If you can't translate something
   properly, say so in the Turkish file rather than leaving it silently out of date — an assistant that reads
   Turkish can usually do this translation step for you.
5. If your change touches a UI string, add it to **all seven** dictionaries in `web/src/i18n/` (`tr.ts` defines the
   shape; `npm run check` fails if any language is missing a key). You don't need to speak all seven languages
   yourself — an AI assistant translating into a language you don't speak is fine, but please have it (or a
   speaker, if you have one) sanity-check tone and that proper nouns like SuperUser/ACL/Opus stay in Latin script,
   matching the existing dictionaries.
6. Open the pull request against `main`. Describe what changed and why; link the issue it resolves, if any.

## What a review looks for

- Correctness and test coverage, including the Go integration tests in `gateway/internal/bridge` where relevant
  (they need a running Mumble server, see [docs/BUILDING.md](docs/BUILDING.md)).
- No security regression: the gateway only ever connects to the one configured Mumble address, Origin/session/rate
  limits stay intact, nothing sensitive (passwords, certificate keys, chat content) gets logged. See
  [docs/SECURITY.md](docs/SECURITY.md) for the full threat model.
- Documentation kept in sync, in both languages, as described above.
- Scope: a focused change over a sprawling one. If a PR grew beyond its original issue, it's fine to split it.

## Code of conduct

Be respectful and assume good faith, including toward contributions you can tell were AI-assisted — that's most of
this codebase too. Disagreements about an approach belong in the issue or PR thread, on the technical merits.
