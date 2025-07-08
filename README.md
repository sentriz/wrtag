![wrtag](https://share.senan.xyz/wrtag.svg)

_Fast automated music tagging and organisation based on MusicBrainz_

---

**wrtag** is similar to music tagging and organisation tools such as [Beets](https://beets.io/) and [MusicBrainz Picard](https://picard.musicbrainz.org/) but aims to be simpler, more composable, and faster.

To describe the general workflow:

1. Input files are provided (existing or new).
2. They are matched with releases in the [MusicBrainz database](https://musicbrainz.org/). This may be done using existing tags or filenames.
3. The files are moved or copied to a user-defined filesystem layout. For example, `My music/Artist name/(Release year) Release name/1. Track title.ext`.
4. The file's tags are updated to match the tags in the MusicBrainz database.
5. Enjoy a clean, consistent, and accurate browsing experience in your music player/server of choice.

# Table of Contents

1. [Features](#features)
2. [Included tools](#included-tools)
   - [Tool `wrtag`](#tool-wrtag)
     - [Importing new music](#importing-new-music)
     - [Re-tagging already imported music](#re-tagging-already-imported-music)
     - [Available operations](#available-operations)
   - [Tool `wrtagweb`](#tool-wrtagweb)
     - [API](#api)
     - [Configuration](#configuration)
     - [Options](#options)
   - [Tool `metadata`](#tool-metadata)
3. [Installation](#installation)
4. [Global configuration](#global-configuration)
   - [Options](#options-1)
   - [Format](#format)
     - [CLI arguments](#cli-arguments)
     - [Environment variables](#environment-variables)
     - [Config file](#config-file)
5. [Path format](#path-format)
   - [Basic structure](#basic-structure)
   - [Available template data](#available-template-data)
   - [Helper functions](#helper-functions)
   - [Example formats](#example-formats)
6. [Addons](#addons)
   - [Addon Lyrics](#addon-lyrics)
   - [Addon ReplayGain](#addon-replaygain)
   - [Addon Subprocess](#addon-subprocess)
   - [Addon Music descriptors](#addon-music-descriptors)
7. [Tagging](#tagging)
   - [Tags written](#tags-written)
   - [Tags kept by default](#tags-kept-by-default)
   - [Tag configuration](#tag-configuration)
8. [Notifications](#notifications)
9. [Goals and non-goals](#goals-and-non-goals)

# Features

- Unix-style suite of tools for different use cases, using the same core **wrtag** functionality.
- **Fast** tagging thanks to [Go](https://go.dev/).
- Filesystem organisation of music files, covers, and configurable extra files.
- **Cover fetching** or upgrades from the [Cover Art Archive](https://coverartarchive.org/).
- Care taken to ensure **no orphan** folders are left in the library when moves or copies occur.
- Validation to ensure your library is **always consistent** with no duplicates or unrecognised paths.
- Safe **concurrent** processing with tree-style filesystem locking.
- Addons for fetching lyrics, calculating [ReplayGain](https://wiki.hydrogenaud.io/index.php?title=ReplayGain_2.0_specification), or any user-defined subprocess.
- Rescanning the library and processing it for new changes in MusicBrainz (`wrtag sync`).
- An optional **web interface** for importing new releases over the network. Allows the user to be notified and confirm details if there is no 100% match found.
- Support for [gazelle-origin](https://github.com/x1ppy/gazelle-origin) files to improve matching from certain sources.
- Support for **Linux**, **macOS**, and **Windows** with static/portable [binaries available](https://github.com/sentriz/wrtag/releases) for each.

# Included tools

## Tool `wrtag`

<img align="right" width="300" src=".github/screenshot-wrtag-copy.png">

The `wrtag` tool is the main command line tool for music organisation. It has subcommands for working on individual folders (e.g. `copy`, `move`), and one for bulk processing (`sync`).

### Importing new music

#### Moving from source

`wrtag` is the main command line tool that works on a single folder and requires an operation such as `move` or `copy`.

For example:

```console
$ wrtag move "Example"                 # tags and moves `Example` into the library defined by the path-format config option
$ wrtag move -dry-run "Example"        # shows move and tag operations without applying them
$ wrtag move -yes "Example"            # use anyway even if low match
$ wrtag move -mbid "abc" -yes "Example" # overwrite matched MusicBrainz release UUID
```

#### Copying from source

If the source files should be left alone, `wrtag` also provides a `copy` operation:

```console
$ wrtag copy -yes "Example"      # copies and tags `Example` into the library, use anyway even if low match
$ wrtag copy -mbid "abc" -yes    # overwrite matched MusicBrainz release UUID even if low match
```

### Re-tagging already imported music

Re-tagging your music can be useful for a few reasons. For example, if your [path-format](#path-format) configuration has changed, or the metadata in the MusicBrainz database has changed.

#### Re-tagging a single release

Since a `move` operation always cleans up the source directory - and is smart about the music already being in place - a re-tag is just a move pointed at some music that is already in place:

```console
# path-format is /my/music/XXX
$ wrtag move "/my/music/Tame Impala/(2010) Innerspeaker"
# now has updated tags, and moved again if needed
```

### Available operations

The full list of core `wrtag` operations. They can be used in other tools like `wrtagweb` too.

| Name      | Description                                                                                                     |
| --------- | --------------------------------------------------------------------------------------------------------------- |
| `move`    | Moves files from the source to the destination directory.                                                       |
| `copy`    | Copies files from the source to the destination directory.                                                      |
| `reflink` | On supported filesystems, creates a reflink (copy-on-write) clone of a file from the source to the destination. |

#### Re-tagging in bulk

Bulk operations are done with the `sync` subcommand. Unlike the `copy` and `move` commands which operate on single releases, the `sync` command works on your already imported and tagged library.

> [!WARNING]
> As the `sync` command is non-interactive, when used incorrectly it can be destructive. Only use `sync` on a library whose contents have been populated by `copy` or `move`.

By default, `sync` recurses through all directories rooted in `path-format` and finds leaf directories. A leaf directory is one that has no sub-folders, and therefore looks like a release. The tracks are read, and if they have a `MUSICBRAINZ_ALBUMID` (e.g., from `copy` or `move`), the release info is fetched from MusicBrainz and the release is re-tagged.

If no `MUSICBRAINZ_ALBUMID` is present, the release is matched as it usually would, and only re-tagged if a high match score is calculated.

```console
$ wrtag sync                          # recurse all releases and re-tag
$ wrtag sync -dry-run                 # show what above would do
$ wrtag sync "/my/music/Tame Impala"  # find all releases in "Tame Impala/" and re-tag those
$ wrtag sync -age-older 24h           # find all releases older than 1 day and re-tag
$ wrtag sync -num-workers 16          # process a maximum of 16 releases at a time
```

## Tool `wrtagweb`

<img align="right" width="300" src=".github/screenshot-wrtagweb.png">

`wrtagweb` is based on the same core functionality as `wrtag`, except it's web-based instead of command line. Instead of importing releases from the command line arguments, new releases are imported over HTTP.

For example, an HTTP client (a custom script, a BitTorrent client "on complete" script, or Soulseek) sends an HTTP request to `wrtagweb`, giving it a new path to import. `wrtagweb` imports it. If there isn't a 100% match, the user is notified. Then, the user can correct the match, reject, or accept anyway.

```mermaid
%%{init: {'theme': 'base', 'themeVariables': { 'fontSize': '26px' }}}%%
flowchart LR
    a["BitTorrent / Soulseek script"] -->
    b["Release added to queue"] -->
    c["User notified if input needed"] -->
    d["Release imported"]
```

### API

Jobs are added to the queue with an HTTP request like `POST <wrtag.host>/op/<copy|move>` with form value `path=<absolute path to directory>`. The form value can be an `application/x-www-form-urlencoded` form body, or URL query parameter.

Authentication is done via a HTTP Basic authentication password **without a username**. The password is configured with the `web-api-key` config option.

> [!WARNING]
> HTTP Basic Authentication is only as secure as the transport layer it runs on. Make sure `wrtagweb` is secured using TLS behind your reverse proxy.

<details>
<summary><b>Example with <i>cURL</i></b></summary>

```console
curl \
    --request POST \
    --data-urlencode "path=/path/to/the/release" \
    "https://:my-api-key@wrtag.hostname/op/copy"
```

</details>

<details>
<summary><b>Example with <i>Transmission</i></b></summary>

Create a script named `done.sh` or anything you like, and make it executable:
`chmod +x done.sh`

Update your Transmission `settings.json` to reference the new script:

```json
...
"script-torrent-done-enabled": true,
"script-torrent-done-filename": "/scripts/done.sh",
...
```

Edit the script to send a `copy` job with the newly finished torrent. Transmission will set `TR_TORRENT_NAME` to the name/path of the torrent. [See all variables](https://github.com/transmission/transmission/blob/main/docs/Scripts.md#on-torrent-completion)

```bash
#!/bin/sh

curl \
    --request POST \
    --data-urlencode "path=<path to downloads>/$TR_TORRENT_NAME" \
    "http://:<wrtag api key>@<wrtag host>/op/copy"
```

</details>

<details>
<summary><b>Example with <i>qBittorrent</i></b></summary>

> TODO

</details>

<details>
<summary><b>Example with <i>Deluge</i></b></summary>

> TODO

</details>

<details>
<summary><b>Example with <i>sldkd</i></b></summary>

> TODO

</details>

### Configuration

Configuration for `wrtagweb` works the same as [Global configuration](#global-configuration). For example, `wrtagweb -web-arg`, `WRTAG_WEB_ARG`, and the global config file is also read.

### Options

<!-- gen with ```go run ./cmd/wrtagweb -h 2>&1 | ./gen-docs | wl-copy``` -->

| CLI argument     | Environment variable  | Config file key | Description                                        |
| ---------------- | --------------------- | --------------- | -------------------------------------------------- |
| -web-api-key     | WRTAG_WEB_API_KEY     | web-api-key     | API key for web interface                          |
| -web-db-path     | WRTAG_WEB_DB_PATH     | web-db-path     | Path to database path for web interface (optional) |
| -web-listen-addr | WRTAG_WEB_LISTEN_ADDR | web-listen-addr | Listen address for web interface                   |
| -web-public-url  | WRTAG_WEB_PUBLIC_URL  | web-public-url  | Public URL for web interface (optional)            |

## Tool `metadata`

<img align="right" width="300" src=".github/screenshot-metadata.png">

The metadata tool is a standalone helper program for reading and writing track metadata. It can write multiple tags (each with multiple values) to multiple files in a single invocation.

Since it uses the same tag normalisation as `wrtag` itself, it works well with the `subproc` addon. This allows for custom metadata read and write after the main release process has completed.

### Usage

```console
  $ metadata [<options>] read  <tag>... -- <path>...
  $ metadata [<options>] write ( <tag> <value>... , )... -- <path>...
  $ metadata [<options>] clear <tag>... -- <path>...

  # <tag> is an audio metadata tag key
  # <value> is an audio metadata tag value
  # <path> is path(s) to audio files, dir(s) to find audio files in, or "-" for list audio file paths from stdin
```

### Examples

```console
  $ metadata read -- a.flac b.flac c.flac
  $ metadata read artist title -- a.flac
  $ metadata read -properties -- a.flac
  $ metadata read -properties title length -- a.flac
  $ metadata write album "album name" -- x.flac
  $ metadata write artist "Sensient" , genres "psy" "minimal" "techno" -- dir/*.flac
  $ metadata write artist "Sensient" , genres "psy" "minimal" "techno" -- dir/
  $ metadata clear -- a.flac
  $ metadata clear lyrics artist_credit -- *.flac
  $ find x/ -type f | metadata write artist "Sensient" , album "Blue Neevus" -
  $ find y/ -type f | metadata read artist title -
  $ find y/ -type f -name "*extended*" | metadata read -properties length -
```

For more, see `metadata -h` and `metadata read -h`

# Installation

## Download a release

You can find static/portable binaries (wrtag, wrtagweb, metadata) on the [releases page](https://github.com/sentriz/wrtag/releases) for Windows, macOS, and Linux.

## Build from source

To install from source, install a recent [Go](https://go.dev/) toolchain, clone the repo, and run `go install ./cmd/...` from inside.

For packagers, CGO is _not_ required, so you can build with `CGO_ENABLED=0` to produce a static binary.

## Using Docker

Docker images for many architectures are available on [Docker Hub](https://hub.docker.com/r/sentriz/wrtag) and [GitHub](https://github.com/sentriz/wrtag/pkgs/container/wrtag). The Docker image by default starts `wrtagweb`, but has the `wrtag` tools included too.

If you're using Docker Compose and `wrtagweb`, you can use this `compose.yml` to get started:

```yaml
services:
  wrtag:
    image: sentriz/wrtag
    environment:
      - WRTAG_WEB_API_KEY= # change this
      - WRTAG_WEB_LISTEN_ADDR=:80
      - WRTAG_WEB_PUBLIC_URL=https://wrtag.example.com
      - WRTAG_WEB_DB_PATH=/data/wrtag.db
      - WRTAG_LOG_LEVEL=debug
      # add more config options, like mentioned in "Global configuration" docs
      # - WRTAG_PATH_FORMAT=...
      # - WRTAG_ADDON=...,...
      # - WRTAG_RESEARCH_LINK=...,...
      # or, use the config file if you use wrtag outside the container. make sure to add it to `volumes:` too
      # - WRTAG_CONFIG_PATH=/config
    expose:
      - 80
    volumes:
      - ./data:/data # for the wrtagweb job queue DB
      - /path/to/music:/path/to/music
```

# Global configuration

Global configuration is used by all tools. Any option can be provided with a CLI argument, environment variable, or config file key. See [Format](#format) for more technical details.

### Options

<!-- gen with ```go run ./cmd/wrtag -h 2>&1 | ./gen-docs | wl-copy``` -->

| CLI argument      | Environment variable   | Config file key  | Description                                                                                          |
| ----------------- | ---------------------- | ---------------- | ---------------------------------------------------------------------------------------------------- |
| -addon            | WRTAG_ADDON            | addon            | Define an addon for extra metadata writing (see [Addons](#addons)) (stackable)                       |
| -caa-base-url     | WRTAG_CAA_BASE_URL     | caa-base-url     | CoverArtArchive base URL (default "<https://coverartarchive.org/>")                                  |
| -caa-rate-limit   | WRTAG_CAA_RATE_LIMIT   | caa-rate-limit   | CoverArtArchive rate limit duration                                                                  |
| -config           | WRTAG_CONFIG           | config           | Print the parsed config and exit                                                                     |
| -config-path      | WRTAG_CONFIG_PATH      | config-path      | Path to config file (default "$XDG_CONFIG_HOME/wrtag/config")                                        |
| -cover-upgrade    | WRTAG_COVER_UPGRADE    | cover-upgrade    | Fetch new cover art even if it exists locally                                                        |
| -keep-file        | WRTAG_KEEP_FILE        | keep-file        | Define an extra file path to keep when moving/copying to root dir (stackable)                        |
| -log-level        | WRTAG_LOG_LEVEL        | log-level        | Set the logging level (default INFO)                                                                 |
| -mb-base-url      | WRTAG_MB_BASE_URL      | mb-base-url      | MusicBrainz base URL (default "<https://musicbrainz.org/ws/2/>")                                     |
| -mb-rate-limit    | WRTAG_MB_RATE_LIMIT    | mb-rate-limit    | MusicBrainz rate limit duration (default 1s)                                                         |
| -notification-uri | WRTAG_NOTIFICATION_URI | notification-uri | Add a shoutrrr notification URI for an event (see [Notifications](#notifications)) (stackable)       |
| -path-format      | WRTAG_PATH_FORMAT      | path-format      | Path to root music directory including path format rules (see [Path format](#path-format))           |
| -research-link    | WRTAG_RESEARCH_LINK    | research-link    | Define a helper URL to help find information about an unmatched release (stackable)                  |
| -diff-weight      | WRTAG_DIFF_WEIGHT      | diff-weight      | Adjust distance weighting for a tag when diffing (0 to ignore) (stackable)                           |
| -tag-config       | WRTAG_TAG_CONFIG       | tag-config       | Specify tag keep and drop rules when writing new tag revisions (see [Tagging](#tagging)) (stackable) |
| -version          | WRTAG_VERSION          | version          | Print the version and exit                                                                           |

### Format

### CLI arguments

Just call the command with the CLI argument. For example, `wrtag -some-key "some value"`. For stackable (repeatable) arguments, pass them multiple times. For example, `wrtag -some-key "value 1" -some-key "value 2"`.

> [!NOTE]
> Be aware of the position of global vs command arguments. For example, `wrtag <global options> cmd <cmd options>`. Check `-h` when in doubt.

### Environment variables

Environment variables are prefixed with `WRTAG_` usually. For example, `WRTAG_LOG_LEVEL=info wrtag`. For stackable (repeatable) arguments, join them with a comma (`,`). For example, `WRTAG_ADDON="replaygain,lyrics genius musixmatch"`. If the value of the variable should have a comma, it can be escaped with a backslash. For example, `\,`.

### Config file

The config file can be used instead of CLI arguments or environment variables, but can be overwritten with the `-config-path` CLI argument or `WRTAG_CONFIG_PATH` environment variable.

Note the default config file locations:

| OS      | Path                                             |
| ------- | ------------------------------------------------ |
| Linux   | `$XDG_CONFIG_HOME/wrtag/config`                  |
| Windows | `%AppData%\wrtag\config`                         |
| macOS   | `$HOME/Library/Application Support/wrtag/config` |

The format follows ([flagconf](https://pkg.go.dev/go.senan.xyz/flagconf?utm_source=godoc#hdr-usage)), which looks something like:

```
some-key argument
other-key argument
```

For stackable (repeatable) arguments, provide the same key multiple times. For example:

```
addon replaygain
addon lyrics genius musixmatch
```

See the [example config](./config.example) for more.

# Path format

The `path-format` configuration option defines both the root music directory and the template for organising your music files. This template uses Go's text/template syntax and is populated with MusicBrainz release data.

## Basic structure

In order to minimise potential release conflict, the path format should include at least three path segments:

1. The root music directory (where all your music will be stored).
2. Artist/release organisation (typically artist name and album details).
3. Track naming format (including track numbers and titles).

For example:

```
path-format /music/library/{{ <some artist format> }}/({{ <some release format> }}/{{ <track format> }}
```

This could format a release like:

```
/music/library/Tame Impala/(2010) Innerspeaker/01. It Is Not Meant to Be.flac
/music/library/Tame Impala/(2010) Innerspeaker/02. Desire Be Desire Go.flac
/music/library/Tame Impala/(2010) Innerspeaker/03. Alter Ego.flac
...
```

On Windows, you can use drive letters and backslashes:

```
path-format C:\User\John\Music\{{ <some artist format> }}\({{ <some release format> }}\{{ <track format> }}
```

## Available template data

The template has access to the following data:

- `.Release` - The full MusicBrainz release object (see [`type Release struct {`](https://github.com/sentriz/wrtag/blob/master/musicbrainz/musicbrainz.go))
- `.Track` - The current track being processed (see [`type Track struct {`](https://github.com/sentriz/wrtag/blob/master/musicbrainz/musicbrainz.go))
- `.TrackNum` - The track number (integer, starting at 1)
- `.Tracks` - The list of tracks in the release
- `.ReleaseDisambiguation` - A string for release and release group disambiguation
- `.IsCompilation` - Boolean indicating if this is a compilation album
- `.Ext` - The file extension for the current track, including the dot (e.g., ".flac")

## Helper functions

In addition to what's provided by Go [text/template](https://pkg.go.dev/text/template), several helper functions are available to format your paths:

| Function              | Description                                                | Example                                                      |
| --------------------- | ---------------------------------------------------------- | ------------------------------------------------------------ |
| `join`                | Joins strings with a delimiter                             | `{{ artists .Release.Artists \| join "; " }}`                |
| `pad0`                | Zero-pads a number to specified width                      | `{{ pad0 2 .TrackNum }}` → "01"                              |
| `sort`                | Sorts a string array                                       | `{{ artists .Release.Artists \| sort }}`                     |
| `the`                 | Moves `A` and `The` to the end of each string in the input | `{{ artists .Release.Artists \| sort \| the \| join "; " }}` |
| `safepath`            | Makes a string safe for filesystem use                     | `{{ .Track.Title \| safepath }}`                             |
| `artists`             | Gets artist names from artist credits                      | `{{ artists .Release.Artists }}`                             |
| `artistsString`       | Formats artists as a string                                | `{{ artistsString .Track.Artists }}`                         |
| `artistsEn`           | Gets artist names in English locale from artist credits    | `{{ artistsEn .Release.Artists }}`                           |
| `artistsEnString`     | Formats artists names in English locale as a string        | `{{ artistsEnString .Track.Artists }}`                       |
| `artistsCredit`       | Gets credit names from artist credits                      | `{{ artistsCredit .Release.Artists }}`                       |
| `artistsCreditString` | Formats artist credits as a string                         | `{{ artistsCreditString .Release.Artists }}`                 |
| `artistsSort`         | Gets sort names from artist credits                        | `{{ artistsSort .Release.Artists \| sort \| join "; " }}`    |
| `artistsSortString`   | Formats artist sort names as a credits                     | `{{ artistsSortString .Release.Artists }}`                   |

## Example formats

> [!NOTE]
> If you need help with creating custom path formats, please see the provided playground [here](https://go.dev/play/p/hbmSFWoq81G)

### The recommended format

Including multi-album artist support, release group year, release group and release disambiguations, track numbers, total track numbers, artist names if the release is a compilation album:

```
/music/{{ artists .Release.Artists | sort | join "; " | safepath }}/({{ .Release.ReleaseGroup.FirstReleaseDate.Year }}) {{ .Release.Title | safepath }}{{ if not (eq .ReleaseDisambiguation "") }} ({{ .ReleaseDisambiguation | safepath }}){{ end }}/{{ pad0 2 .TrackNum }}.{{ len .Tracks | pad0 2 }} {{ if .IsCompilation }}{{ artistsString .Track.Artists | safepath }} - {{ end }}{{ .Track.Title | safepath }}{{ .Ext }}
```

### A basic format

```
/music/{{ artists .Release.Artists | join "; " | safepath }}/{{ .Release.Title | safepath }}/{{ pad0 2 .TrackNum }} {{ .Track.Title | safepath }}{{ .Ext }}
```

### With year and disambiguation

```
/music/{{ artists .Release.Artists | sort | join "; " | safepath }}/({{ .Release.ReleaseGroup.FirstReleaseDate.Year }}) {{ .Release.Title | safepath }}{{ if not (eq .ReleaseDisambiguation "") }} ({{ .ReleaseDisambiguation | safepath }}){{ end }}/{{ pad0 2 .TrackNum }} {{ .Track.Title | safepath }}{{ .Ext }}
```

### With compilation handling

```
/music/{{ artists .Release.Artists | sort | join "; " | safepath }}/({{ .Release.ReleaseGroup.FirstReleaseDate.Year }}) {{ .Release.Title | safepath }}/{{ pad0 2 .TrackNum }} {{ if .IsCompilation }}{{ artistsString .Track.Artists | safepath }} - {{ end }}{{ .Track.Title | safepath }}{{ .Ext }}
```

### With disc and track numbers

```
/music/{{ artists .Release.Artists | sort | join "; " | safepath }}/({{ .Release.ReleaseGroup.FirstReleaseDate.Year }}) {{ .Release.Title | safepath }}/{{ pad0 2 .TrackNum }}.{{ len .Tracks | pad0 2 }} {{ .Track.Title | safepath }}{{ .Ext }}
```

# Addons

Addons can be used to fetch/compute additional metadata after the MusicBrainz match has been applied and the files have been tagged.

They are configured as part of the [global configuration](#global-configuration) using a [config format](#format).

For example:

- `$ wrtag -addon "lyrics a b c" -addon "replaygain x y z"`
- `$ WRTAG_ADDON="lyrics a b c,replaygain" wrtag`
- or repeating the `addon` clause in the config file.

## Addon Lyrics

The `lyrics` addon can fetch and embed lyric information from [Genius](https://genius.com/) and [Musixmatch](https://www.musixmatch.com/) in your tracks.

The format of the addon config is `lyrics <source>...` where the source is one of `genius` or `musixmatch`. For example, `"lyrics genius musixmatch"`. Note that sources will be tried in the order they are specified.

## Addon ReplayGain

The `replaygain` addon computes and adds [ReplayGain 2.0](https://wiki.hydrogenaud.io/index.php?title=ReplayGain_2.0_specification) information to your files. It is great for normalising the perceived loudness of audio in your tracks.

> [!NOTE]
> The ReplayGain addon requires the [`rsgain`](https://github.com/complexlogic/rsgain) program to available in your `$PATH`

The format of the addon config is `replaygain <opts>...` where opts can be `true-peak` and `force`. If the force option is passed, ReplayGain information is recomputed even if it’s already present in the files.

## Addon Subprocess

The subprocess addon is for running a user-provided program.

> [!NOTE]
> If your subprocess ends up writing tags, for example with the `metadata` command, then you proably want to configure a [`keep` rule](#tag-configuration) for the written tags. Otherwise, wrtag will clear it only for your addon to rewrite it again after every operation.

The format of the addon config is `subproc <path> <args>...`, where `path` is the path to the program, or the program name itself if it’s in your `$PATH`. `args` are extra command line arguments to pass to the program. One of the `args` should be a special placeholder named `<files>`. This will be expanded to the paths to the files that were just processed by `wrtag`.

For example, the addon `"subproc my-program a --b 'c d' <files>"` might call `my-program` with arguments `["a", "--b", "c d", "track 1.flac", "track 2.flac", "track 3.flac"]` after importing a release with 3 tracks.

## Addon Music descriptors

The `musicdesc` addon can calculate and embed music descriptors information using [`streaming_extractor_music`](https://essentia.upf.edu/streaming_extractor_music.html) from [Essentia](https://essentia.upf.edu/index.html).

> [!NOTE]
> The Music descriptors addon requires that `streaming_extractor_music` be available in your `$PATH`. See their [extractors](https://essentia.upf.edu/extractors/) download page.

The format of the addon config is `musicdesc <opts>...` where opts can be only `force`. If the force option is passed, descriptor information is recomputed even if it’s already present in the files.

Tags currently written are

| Tag          | Example Value       |
| ------------ | ------------------- |
| `INITIALKEY` | `C`, `F#`, or `Dbm` |
| `BPM`        | `120.21`            |

# Tagging

During the music tagging process, **wrtag** follows a systematic approach to ensure consistent and accurate metadata

1. **All existing tags are cleared** from the audio files to start with a clean slate
2. **Main tags are written** using data from the MusicBrainz release (see [Tags written](#tags-written))
3. **Default keep tags are preserved** from the original files (see [Tags kept by default](#tags-kept-by-default))
4. **Custom `tag-config` rules are applied** to further customise the final tag set (see [Tag configuration](#tag-configuration))

## Tags written

The `Tag` column's strings are mapped their respective container format if possible. If not, the container format's support for custom tags is used. See [Mapping of Properties to Various Formats](https://taglib.org/api/p_propertymapping.html).

Example track is [America Is Waiting](https://musicbrainz.org/release/3b28412d-8a47-3da9-8331-525947231d50/disc/1#c0b83973-4f74-3200-8411-392630d9c945) from a 2005 remaster of [My Life in the Bush of Ghosts](https://musicbrainz.org/release/3b28412d-8a47-3da9-8331-525947231d50)

> [!NOTE]
> The artist credit names on that release are the same as their main names, so changes where made to make the distinction clear.

| Tag                          | Description                                               | Example Value                                                                                                                                                                                                                |
| ---------------------------- | --------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ALBUM`                      | Release name                                              | `My Life in the Bush of Ghosts`                                                                                                                                                                                              |
| `ALBUMARTIST`                | Release artist as string                                  | `Brian Eno + David Byrne`                                                                                                                                                                                                    |
| `ALBUMARTISTS`               | Release artist names as multi-valued tag                  | `Brian Eno`, `David Byrne`                                                                                                                                                                                                   |
| `ALBUMARTIST_CREDIT`         | Release artist credit as string                           | `Eno + Byrne`                                                                                                                                                                                                                |
| `ALBUMARTISTS_CREDIT`        | Release artist credit names as multi-valued tag           | `Eno`, `Byrne`                                                                                                                                                                                                               |
| `DATE`                       | Release date                                              | `2006-03-27`                                                                                                                                                                                                                 |
| `ORIGINALDATE`               | Original release date                                     | `1981-02-01`                                                                                                                                                                                                                 |
| `MEDIA`                      | Media format                                              | `Enhanced CD`                                                                                                                                                                                                                |
| `LABEL`                      | Record label                                              | `Virgin`                                                                                                                                                                                                                     |
| `CATALOGNUMBER`              | Catalogue number                                          | `BEDBX 1`                                                                                                                                                                                                                    |
| `BARCODE`                    | Release barcode/UPC                                       | `0094633134126`                                                                                                                                                                                                              |
| `COMPILATION`                | Release has type compilation / is Various Artists         | `1`                                                                                                                                                                                                                          |
| `RELEASETYPE`                | Release primary type                                      | `album`                                                                                                                                                                                                                      |
| `TITLE`                      | Track title                                               | `America Is Waiting`                                                                                                                                                                                                         |
| `ARTIST`                     | Track artist as string                                    | `Brian Eno + David Byrne`                                                                                                                                                                                                    |
| `ARTISTS`                    | Track artist names as multi-valued tag                    | `Brian Eno`, `David Byrne`                                                                                                                                                                                                   |
| `ARTIST_CREDIT`              | Track artist credit as string                             | `Eno + Byrne`                                                                                                                                                                                                                |
| `ARTISTS_CREDIT`             | Track artist credit names as multi-valued tag             | `Eno`, `Byrne`                                                                                                                                                                                                               |
| `GENRE`                      | Primary genre                                             | `ambient`                                                                                                                                                                                                                    |
| `GENRES`                     | Genre list as multi-valued tag                            | `ambient`, `art rock`, `electronic`, `experimental`                                                                                                                                                                          |
| `TRACKNUMBER`                | Track number                                              | `1`                                                                                                                                                                                                                          |
| `ISRC`                       | International Standard Recording Code                     | `GBAAA0500384`                                                                                                                                                                                                               |
| `REMIXER`                    | Concatenated remixers on the recording                    | `Artist One, Artist Two`                                                                                                                                                                                                     |
| `REMIXERS`                   | multi-valued remixers on the recording                    | `Artist One`, `Artist Two`                                                                                                                                                                                                   |
| `REMIXER_CREDIT`             | Concatenated remixers on the recording, credited name     | `artist.one, artist.two`                                                                                                                                                                                                     |
| `REMIXERS_CREDIT`            | multi-valued remixers on the recording, credited name     | `artist.one`, `artist.two`                                                                                                                                                                                                   |
| `COMPOSER`                   | Concatenated composers on the related work                | `Artist One, Artist Two`                                                                                                                                                                                                     |
| `COMPOSERS`                  | multi-valued composers on the related work                | `Artist One`, `Artist Two`                                                                                                                                                                                                   |
| `COMPOSER_CREDIT`            | Concatenated composers on the related work, credited name | `artist.one, artist.two`                                                                                                                                                                                                     |
| `COMPOSERS_CREDIT`           | multi-valued composers on the related work, credited name | `artist.one`, `artist.two`                                                                                                                                                                                                   |
| `MUSICBRAINZ_ALBUMID`        | MusicBrainz release ID                                    | [`3b28412d-8a47-3da9-8331-525947231d50`](https://musicbrainz.org/release/3b28412d-8a47-3da9-8331-525947231d50)                                                                                                               |
| `MUSICBRAINZ_RELEASEGROUPID` | MusicBrainz release group ID                              | [`0dd5a352-6ae0-3e0d-bf18-bec30e27807d`](https://musicbrainz.org/release-group/0dd5a352-6ae0-3e0d-bf18-bec30e27807d)                                                                                                         |
| `MUSICBRAINZ_ALBUMARTISTID`  | MusicBrainz release artist IDs                            | [`ff95eb47-41c4-4f7f-a104-cdc30f02e872`](https://musicbrainz.org/artist/ff95eb47-41c4-4f7f-a104-cdc30f02e872), [`d4659efb-b8eb-4f03-95e9-f69ce35967a9`](https://musicbrainz.org/artist/d4659efb-b8eb-4f03-95e9-f69ce35967a9) |
| `MUSICBRAINZ_ALBUMCOMMENT`   | MusicBrainz release and release group disambiguation      | `2005 remaster`                                                                                                                                                                                                              |
| `MUSICBRAINZ_TRACKID`        | MusicBrainz recording ID                                  | [`28a9587e-15a1-49d6-b02a-65d29b748ffe`](https://musicbrainz.org/recording/28a9587e-15a1-49d6-b02a-65d29b748ffe)                                                                                                             |
| `MUSICBRAINZ_RELEASETRACKID` | MusicBrainz track ID                                      | [`c0b83973-4f74-3200-8411-392630d9c945`](https://musicbrainz.org/track/c0b83973-4f74-3200-8411-392630d9c945)                                                                                                                 |
| `MUSICBRAINZ_ARTISTID`       | MusicBrainz track artist ID                               | [`ff95eb47-41c4-4f7f-a104-cdc30f02e872`](https://musicbrainz.org/artist/ff95eb47-41c4-4f7f-a104-cdc30f02e872), [`d4659efb-b8eb-4f03-95e9-f69ce35967a9`](https://musicbrainz.org/artist/d4659efb-b8eb-4f03-95e9-f69ce35967a9) |

## Tags kept by default

The following tags are automatically preserved from the original files during the tagging process, if present

- ReplayGain settings
- BPM, Key, Lyrics
- AcoustID identifiers
- Encoder comments

## Tag configuration

The `tag-config` option allows you to customise which tags are kept or dropped during the tagging process. This option can be used multiple times and supports two operations

- `keep <tag>` - Preserve the specified tag from the original file
- `drop <tag>` - Remove the specified tag from the final output

The option is configured as part of the [global configuration](#global-configuration) using a [config format](#format).

For example:

- `$ wrtag -tag-config "keep CUSTOM_RATING" -tag-config "keep MOOD" -tag-config "drop GENRE"`
- `$ WRTAG_TAG_CONFIG="keep CUSTOM_RATING,keep MOOD,drop GENRE,drop COMMENT"`
- or repeating the `tag-config` clause in the config file

This example configuration would

- Keep any existing `CUSTOM_RATING` and `MOOD` tags from the original files
- Remove `GENRE` tags from the final output (even though **wrtag** normally writes genre information)
- Remove `COMMENT` tags from the final output (overriding the default keep behavior)

# Notifications

Notifications can be used to notify you or another system of events such as importing or syncing. For example, sending an email when user input is needed to import a release. Or notifying your [music server](https://github.com/sentriz/gonic) after a sync has completed.

The possible events are:

| Tool       | Name            | Description                                    |
| ---------- | --------------- | ---------------------------------------------- |
| `wrtagweb` | `complete`      | Executed when a release is imported            |
| `wrtagweb` | `needs-input`   | Executed when a release requires input         |
| `wrtag`    | `sync-complete` | Executed when a sync has completed             |
| `wrtag`    | `sync-error`    | Executed when a sync has completed with errors |

`wrtag` uses [shoutrrr](https://github.com/containrrr/shoutrrr) to provide upstream notifications over SMTP, HTTP, etc.

For example:

- `smtp://username:password@host:port/?from=from@example.com&to=recipient@example.com`
- `generic+https://my.subsonic.com/rest/startScan.view?c=wrtag&v=1.16&u=user&p=password`

See [the shoutrrr documentation](https://containrrr.dev/shoutrrr/v0.8/services/overview/) for the list of providers.

Notifications are configured as part of the [global configuration](#global-configuration). The format is `<eventspec> <shoutrrr uri>`. `eventspec` is a comma-separated list of event names.

For example, `"complete,sync-complete smtp://example.com"`. Multiple eventspec and URIs can be configured by stacking the config option according to the [config format](#format).

# Goals and non-goals

> TODO
