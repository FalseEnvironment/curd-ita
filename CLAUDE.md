# curd-ita

Fork of [Wraient/curd](https://github.com/Wraient/curd) that adds the AnimeWorld
provider for Italian audio and Italian subtitles.

## Remotes — read before any git write

| Remote   | Repository                              | Role                     |
| -------- | --------------------------------------- | ------------------------ |
| `fork`   | `FalseEnvironment/curd-ita`              | This fork. Push here.    |
| `origin` | `Wraient/curd`                           | Upstream. Never push.    |

Note that `origin` is the upstream repository, not the fork — the usual naming is
inverted here.

Rules:

- Push only to `fork`. Never push, force-push, or open a pull request against
  `origin` (`Wraient/curd`), and never delete or modify branches or tags there.
- Fetching and reading from `origin` is fine (`git fetch origin`, diffs, logs).
- GitHub's "Compare & pull request" banner defaults to the upstream repository.
  Do not follow it; any pull request belongs inside `FalseEnvironment/curd-ita`.
- Ask before updating `fork/main`. Work lands on feature branches such as
  `animeworld-provider` first.

## Provider defaults

The fork ships `Provider=["animeworld"]` as the default in `defaultConfigMap()`
(`internal/config.go`), so a fresh install plays Italian only, with no fallback to
the English providers. `Provider=stacked` restores the upstream fallback chain.

The AnimeWorld base URL defaults to `https://www.animeworld.ac`
(`internal/providers/animeworld/client.go`). The site rotates its TLD; override it
with the `CURD_ANIMEWORLD_BASE` environment variable rather than editing the
constant.

User config lives at `~/.config/curd/curd.conf`.

## Checks

```bash
go build ./...
go test ./internal/...
```

Live tests hit the real providers and are gated behind `CURD_LIVE=1`.
