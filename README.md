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

<<<<<<< HEAD
You can quit it anytime and the resume time would be saved in the history file

more settings can be found at config file.
config file is located at ```~/.config/curd/curd.conf```

On first start, curd asks which tracking mode you want to use:

- local
- anilist
- myanimelist
- anilist + myanimelist

Local history stays enabled in all modes. `anilist` means local history + AniList sync, `myanimelist` means local history + MyAnimeList sync, and `anilist + myanimelist` updates both platforms. In dual-sync mode, curd compares the latest remote update time for each anime and pushes the newest status/progress/category back to the older tracker so both sides converge automatically. Legacy installs are migrated to `anilist` automatically. If you switch trackers later, curd exposes a **Change Tracker** menu entry and can either merge both lists, replace MyAnimeList with AniList, or replace AniList with MyAnimeList.

MyAnimeList OAuth requires your own MAL application credentials. You can set them in the config file or with environment variables:

```bash
CURD_MAL_CLIENT_ID=your_client_id
CURD_MAL_CLIENT_SECRET=your_client_secret
```

If the browser reaches the localhost callback page but curd does not continue automatically, rerun the command and paste the full callback URL when prompted. Curd now keeps the pending MyAnimeList PKCE state so the login can be completed manually.

| **Option**               | **Type**   | **Valid Values**                           | **Description**                                                                                   |
|---------------------------|------------|-------------------------------------------|---------------------------------------------------------------------------------------------------|
| `DiscordPresence`         | Boolean    | `true`, `false`                           | Enables or disables Discord Rich Presence integration.                                            |
| `AnimeNameLanguage`       | Enum       | `english`, `romaji`                       | Sets the preferred language for anime names.                                                      |
| `MpvArgs`                 | List       | all mpv args eg ["--fullscreen=yes", "--mute=yes"]     | Add args to mpv player                                                               | 
| `MpvPlaybackStartTimeout` | Integer    | `1` to `600` (seconds)                    | How long to wait for MPV playback to start before trying another provider. Default: `20`          |
| `AddMissingOptions`       | Boolean    | `true`, `false`                           | When `true`, on a **version upgrade** only options registered as new for that release are **appended** to `curd.conf` (not every historical default). Set `false` to opt out. Defaults still apply in memory always. |
| `VimKeys`                 | Boolean    | `true`, `false`                           | When `true`, selection menus use vim motions: `j`/`k`/`h`/`l` (and arrows) to move, `/` or `?` to search. Default: `false` (type-to-filter immediately). Add `VimKeys=true` to `~/.config/curd/curd.conf` to enable. |
| `CheckUpdates`            | Boolean    | `true`, `false`                           | When `true` (default), checks GitHub for a newer release **while idle** (does not delay startup). If an update is found, the next launch shows notes and options: update now / skip / remind later / turn off. |
| `MpvEpisodePlaylist`    | Boolean    | `true`, `false`                           | When `true` (default), after playback is stable curd fills the MPV playlist with episodes (and dub/sub when available) so you can pick from MPV's playlist UI. Access: `MPV` → right-click the prev/next OSC button (or `Ctrl+P`). Selecting a row loads that stream and keeps tracking in sync. |
| `AlternateScreen`         | Boolean    | `true`, `false`                           | Toggles the use of an alternate screen buffer for cleaner UI.                                     |
| `RofiSelection`           | Boolean    | `true`, `false`                           | Enables or disables anime selection via Rofi.                                                     |
| `PercentageToMarkComplete`| Integer    | `0` to `100`                              | Sets the percentage of an episode watched to consider it as completed.                            |
| `StoragePath`             | String     | Any valid path (Environment variables accepted)  | Specifies the directory where Curd stores its data.                                        |
| `SubOrDub`                | Enum       | `sub`, `dub`                              | Sets the preferred format for anime audio.                                                        |
| `SubStyle`                | Enum       | `ask`, `soft`, `hard`                     | For AniNeko sub streams: `ask` prompts once when both soft-sub and hard-sub servers exist, then saves your choice here; `soft` uses external `.vtt` subtitles via mpv; `hard` uses burned-in subs. Default: `ask` |
| `NextEpisodePrompt`       | Boolean    | `true`, `false`                           | Prompts the user before automatically playing the next episode.                                   |
| `SubsLanguage`            | String     | `english` (redundant rn)                  | Sets the preferred subtitle language.                                                             |
| `ScoreOnCompletion`       | Boolean    | `true`, `false`                           | Automatically prompts the user to rate the anime upon completion.                                 |
| `SkipOp`                  | Boolean    | `true`, `false`                           | Automatically skips the opening of episodes when supported.                                       |
| `SkipEd`                  | Boolean    | `true`, `false`                           | Automatically skips the ending of episodes when supported.                                        |
| `SkipRecap`               | Boolean    | `true`, `false`                           | Skips recap sections in episodes when supported.                                                  |
| `ImagePreview`            | Boolean    | `true`, `false`                           | Enables or disables image previews during anime selection (only for rofi).                        |
| `Player`                  | String     | any mpv-compatible binary (e.g. `mpv`, `iina`) | Player binary used for playback. If not found, Curd falls back to `mpv`.                          |
| `SaveMpvSpeed`            | Boolean    | `true`, `false`                           | Retains the playback speed set in MPV for next episode.                                           |
| `SkipFiller`              | Boolean    | `true`, `false`                           | Skips filler episodes when supported.                                                             |
| `MenuOrder`               | String     | Comma-separated list                      | Controls which menu items appear and their order. Available options: `CURRENT`, `ALL`, `UNTRACKED`, `UPDATE`, `REMAP_PROVIDER`, `CONTINUE_LAST`, `PLANNING`, `COMPLETED`, `PAUSED`, `DROPPED`, `REWATCHING`, `TRACKER`, `PROVIDER`. Only listed items will be shown. Default: `CURRENT,ALL,UNTRACKED,UPDATE,REMAP_PROVIDER,CONTINUE_LAST,TRACKER,PROVIDER` |
<<<<<<< HEAD
<<<<<<< HEAD
| `Provider`                | List       | `stacked`, `["anipub"]`, `["anineko"]`, `["allanime"]`, `["animepahe"]` | Sets the content-provider fallback list. `stacked` (default) uses the preferred order: senshi → anipub. anineko, allanime and animepahe are disabled by default (site down / anti-bot / Cloudflare) unless included in `Provider`. Default: `stacked` |
=======
| `Provider`                | List       | `stacked`, `["animeworld"]`, `["anipub"]`, `["anineko"]`, `["allanime"]`, `["animepahe"]` | Sets the content-provider fallback list. `stacked` (default) uses the preferred order: animeworld → senshi → anipub → anineko → allanime → animepahe. A single-provider list uses only that site. AllAnime and Animepahe are disabled by default unless included in `Provider`. Default: `stacked` |
>>>>>>> ae78731 (feat: add AnimeWorld provider for Italian subs and dubs)
=======
| `Provider`                | List       | `stacked`, `["animeworld"]`, `["anipub"]`, `["anineko"]`, `["allanime"]`, `["animepahe"]` | Sets the content-provider fallback list. Default in this fork is `["animeworld"]`, which serves Italian subs and dubs only, with no fallback to the English providers. `stacked` uses the preferred order: animeworld → senshi → anipub → anineko → allanime → animepahe. A single-provider list uses only that site. AllAnime and Animepahe are disabled by default unless included in `Provider`. Default: `["animeworld"]` |
>>>>>>> ae504fa (feat: default to the AnimeWorld provider only)
| `ManualProviderSearch`    | Boolean    | `true`, `false`                           | Skip automatic provider matching and always show provider search results for manual selection. Displays a hint with the tracker title, format (TV/Movie/etc.), episode count, and sub/dub mode. Default: `false` |
| `TrackingLocal`           | Boolean    | `true`                                    | Legacy compatibility flag. Local playback history is always enabled.                              |
| `TrackingRemote`          | Enum       | `none`, `anilist`, `myanimelist`, `anilist+myanimelist` | Selects which remote tracker curd syncs with.                                           |
| `TrackingConfigured`      | Boolean    | `true`, `false`                           | Internal flag used to remember that the startup tracking prompt has already been completed.       |
| `MyAnimeListClientID`     | String     | MAL OAuth client ID                       | Client ID used for MyAnimeList browser login.                                                     |
| `MyAnimeListClientSecret` | String     | MAL OAuth client secret                   | Optional secret used for MyAnimeList browser login and token refresh.                             |
| `MyAnimeListImported`     | Boolean    | `true`, `false`                           | Tracks whether the one-time AniList-to-MyAnimeList import prompt has already been handled.        |
=======
On first start curd asks which tracking mode to use: local, `anilist`,
`myanimelist`, or both. MyAnimeList login needs your own OAuth credentials, set in
the config or through `CURD_MAL_CLIENT_ID` and `CURD_MAL_CLIENT_SECRET`.
>>>>>>> f21b953 (docs: trim the README and add CLAUDE.md)

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

<<<<<<< HEAD
This fork ships `Provider=["animeworld"]` as the default, so a fresh install plays
Italian only. To also fall back to the English providers when AnimeWorld has no
match, set `Provider=stacked` in the config.


## Todo (fix)
- Use Powershell for windows token input instead of notepad or cmd
- Add a better way to do commands in windows (Convinience for users)

## Dependencies
- mpv - Video player (required fallback)
- iina - Optional mpv-based player on macOS
- rofi - Selection menu
- ueberzug - Display images in rofi
- chromium - Required for Animepahe (auto-downloaded by default, but Termux users must install manually via `pkg install chromium`)

## API Used
- [Anilist API](https://anilist.gitbook.io/anilist-apiv2-docs) - Update user data and download user data
- [MyAnimeList API](https://myanimelist.net/apiconfig/references/api/v2) - MyAnimeList OAuth and tracking sync
- [AniSkip API](https://api.aniskip.com/api-docs) - Get anime intro and outro timings
- [AllAnime Content](https://allanime.to/) - Fetch anime url
<<<<<<< HEAD
- [Senshi Project](https://senshi.to/) - Default provider with Vidcloud HLS streams and MAL-based catalog matching
=======
- [AnimeWorld](https://www.animeworld.ac/) - Default provider for Italian subs and dubs, serving direct MP4 streams
- [Senshi Project](https://senshi.live/) - Provider with direct HLS streams and MAL-based catalog matching
>>>>>>> ae78731 (feat: add AnimeWorld provider for Italian subs and dubs)
- [AniPub](https://anipub.xyz/) - Fast JSON catalog APIs with MegaPlay HLS streams
- [AniNeko Content](https://anineko.to/) - Alternative provider with soft/hard sub stream selection
- [Animepahe Content](https://animepahe.pw/) - Alternative provider for 1080p streams
- [Jikan](https://jikan.moe/) - Get filler episode number

=======
>>>>>>> f21b953 (docs: trim the README and add CLAUDE.md)
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
