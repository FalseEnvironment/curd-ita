# Curd ITA

A CLI application to stream anime with [AniList](https://anilist.co/) integration and
Discord RPC, written in Go. Works on Linux, MacOS and Windows.

> **This is a fork of [Wraient/curd](https://github.com/Wraient/curd).**
> It adds [AnimeWorld](https://www.animeworld.ac/) as a content provider, the first
> source in curd to serve **Italian audio and Italian subtitles**, and enables it as
> the only provider by default. Everything else behaves like upstream curd.
>
> Upstream release binaries and the AUR `curd` package do **not** contain the
> AnimeWorld provider, so this fork has to be [built from source](#install).

## Features

Everything upstream curd does — AniList/MyAnimeList/local tracking, intro, outro,
filler and recap skipping, Discord RPC, rofi with image previews, resume from where
you left off — plus:

- Italian audio and subtitles through AnimeWorld, matched to AniList by the
  provider's own IDs instead of by title
- Multiple providers with ordered fallback (AnimeWorld, Senshi, AniPub, AniNeko,
  AllAnime, Animepahe) at up to 1080p

Upstream's [README](https://github.com/Wraient/curd#readme) covers the shared
features, demo videos and the community links in more detail.

## Install

Requires Go 1.22 or newer. Runtime dependencies are `mpv`, plus `rofi` and
`ueberzugpp` for the rofi interface.

```bash
git clone https://github.com/FalseEnvironment/curd-ita.git
cd curd-ita
go build -o curd ./cmd/curd
sudo mv curd /usr/local/bin/
```

On Arch Linux the bundled `PKGBUILD` builds the fork and installs it as `curd-ita`,
replacing the AUR `curd` package:

```bash
makepkg -si
```

Nix users can build it with `nix build github:FalseEnvironment/curd-ita`.

## Usage

```bash
curd [options]
```

Command-line arguments always take precedence over the config file.

| Flag              | Description                        |
| ----------------- | ---------------------------------- |
| `-c`              | Continue the last episode          |
| `-new`            | Add a new anime to your list       |
| `-sub` / `-dub`   | Watch the subbed or dubbed version |
| `-rofi`           | Open the selection menu in rofi    |
| `-image-preview`  | Show image previews (rofi only)    |
| `-e`              | Edit the configuration file        |
| `-change-token`   | Change your authentication token   |
| `-u`              | Update the binary                  |
| `-v`              | Show the curd version              |

`curd -h` lists every flag, including the skip toggles (`-skip-op`, `-skip-ed`,
`-skip-filler`, `-skip-recap`), `-player`, `-storage-path` and
`-percentage-to-mark-complete`. Each one maps to a config option of the same name.

## Configuration

Config lives at `~/.config/curd/curd.conf` (`%APPDATA%\Curd` on Windows) and is
editable with `curd -e`. Data — tokens, timestamps, `debug.log`, rofi `.rasi` themes —
lives at `~/.local/share/curd`.

Every option and its valid values are documented in
[upstream's configuration table](https://github.com/Wraient/curd#configuration). The
options that differ in this fork:

| Option     | Valid values                                                      | Description                                                                                                                                             |
| ---------- | ----------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Provider` | `["animeworld"]` (default), `stacked`, or any single-provider list | `["animeworld"]` plays Italian only, with no fallback. `stacked` uses the upstream chain: animeworld → senshi → anipub → anineko → allanime → animepahe. |
| `SubOrDub` | `sub` (default), `dub`                                            | On AnimeWorld, `sub` means Italian subtitles and `dub` means Italian audio.                                                                              |

On first start curd asks which tracking mode to use: local, `anilist`,
`myanimelist`, or both. MyAnimeList login needs your own OAuth credentials, set in
the config or through `CURD_MAL_CLIENT_ID` and `CURD_MAL_CLIENT_SECRET`.

### AnimeWorld (Italian)

AnimeWorld is the only provider enabled by default and streams from
`animeworld.ac`. It behaves differently from the English providers:

- **Subs and dubs are separate catalog entries.** `-dub` moves the dubbed entries to
  the top of the results rather than filtering the subbed ones out, so a series
  without a dub still falls back to the sub.
- **Matching is exact.** The search API returns AniList and MyAnimeList IDs for every
  entry, so curd maps a tracker entry to the right show without guessing by title.
- **Streams are direct MP4 files** from the in-house server, which mpv plays and seeks
  natively. Third-party mirrors are only used when they also expose a direct media
  file.
- **The domain rotates.** AnimeWorld periodically changes its TLD; point curd at the
  current one with `CURD_ANIMEWORLD_BASE=https://www.animeworld.xy` instead of
  rebuilding.

## Credits

- [curd](https://github.com/Wraient/curd) — the upstream project this fork is based on
- [AnimeWorld-API](https://github.com/MainKronos/AnimeWorld-API) — reference for the
  AnimeWorld session and stream flow
- [ani-cli](https://github.com/pystardust/ani-cli) — code for fetching anime URLs
- [jerry](https://github.com/justchokingaround/jerry) — for the inspiration

Curd talks to the [AniList](https://anilist.gitbook.io/anilist-apiv2-docs),
[MyAnimeList](https://myanimelist.net/apiconfig/references/api/v2),
[AniSkip](https://api.aniskip.com/api-docs) and [Jikan](https://jikan.moe/) APIs, and
to the provider sites listed above.
