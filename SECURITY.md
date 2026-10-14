# Security policy

## Reporting a vulnerability

Please report security problems privately, not in a public issue: use GitHub's **Report a vulnerability** button on the
[Security tab](https://github.com/aoztrk82/mumweb/security) of this repository (private vulnerability reporting).
Include the version (`aoztrk82/mumweb` tag), how you run it, and the steps to reproduce.

You can expect an answer within a few days. Fixes go into a new release; the release notes in
[docs/PROGRESS.md](docs/PROGRESS.md) say what was fixed once it is safe to say so.

## Supported versions

Only the latest release (`aoztrk82/mumweb:latest`) gets fixes.

## Scope

In scope: the gateway (`gateway/`), the web client (`web/`), the compose files and Docker image in this repository.
Out of scope: the Mumble server itself (report to the Mumble project), your reverse proxy, and attacks that need the
admin password or control of the host.

The threat model, the controls in place, the review results and the operator recommendations are in
[docs/SECURITY.md](docs/SECURITY.md) ([Türkçe](docs/tr/SECURITY.md)).
