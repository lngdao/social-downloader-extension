# Social-DL

Download videos from 1800+ sites. Chrome extension for scanning + CLI tool for downloading.

## How it works

```
Extension (scan)          CLI (download)

Facebook profile    -->   Paste URLs or load .txt
Instagram page      -->   Concurrent downloads
TikTok, YouTube...  -->   Auto subfolder, retry, history

[Scan Page]               social-dl
[Select All]              > Tai nhieu video
[Copy URLs]               > ctrl+v
                          > ctrl+d
```

**Extension** scans social media pages, collects video URLs, exports to clipboard or `.txt` file.

**CLI** downloads videos using yt-dlp + ffmpeg (auto-installed on first run).

## CLI Tool

### Install

Download the latest binary from [Releases](https://github.com/lngdao/social-dl/releases):

| Platform | File |
|---|---|
| macOS Apple Silicon | `social-dl-macos-apple-silicon` |
| macOS Intel | `social-dl-macos-intel` |
| Windows | `social-dl-windows-amd64.exe` |
| Linux | `social-dl-linux-amd64` |

```bash
# macOS/Linux
chmod +x social-dl-*
./social-dl-macos-apple-silicon
```

First run auto-downloads yt-dlp + ffmpeg (~90MB). On later launches, yt-dlp is
checked and updated automatically at most once every 24 hours. If the update
cannot be completed, the existing yt-dlp binary is kept and the app continues.

### Features

- **Single download** - paste any video URL, choose quality, download
- **Batch download** - paste multiple URLs or load from `.txt` file, concurrent downloads
- **Profile download** - scan YouTube channels, TikTok profiles, playlists
- **Settings** - audio on/off, quality preset, concurrency (1-10), output directory
- **History** - track downloaded videos
- **Auto-update** - checks for new versions on startup
- **Skip metadata** - faster batch downloads with video ID filenames
- **Retry** - failed downloads retry once automatically
- **Download archive** - skip already-downloaded videos

### Supported sites

Everything yt-dlp supports: YouTube, TikTok, Facebook, Instagram, Twitter/X, Reddit, Vimeo, Bilibili, and [1800+ more](https://github.com/yt-dlp/yt-dlp/blob/master/supportedsites.md).

## Chrome Extension

### Install

1. Download `social-dl-extension-vX.X.X.zip` from [Releases](https://github.com/lngdao/social-dl/releases)
2. Unzip
3. Go to `chrome://extensions` > Enable Developer Mode > Load Unpacked > select the unzipped folder

### Usage

1. Open a Facebook/Instagram/TikTok profile page
2. Click the extension icon (opens sidebar)
3. Click **Scan Page** to open the bulk panel
4. Click **Start Scan** - auto-scrolls the page to find videos
5. **Select All** > **Copy URLs** or **Save .txt**
6. Use the CLI tool to batch download

### Extension features

- Auto-scroll scanning with start/pause control
- Select individual videos or select all
- Copy URLs to clipboard
- Save URLs to `.txt` file
- Direct download via extension (single or bulk)
- Quality selection (highest, 1080p, 720p, 360p)

## Development

### Extension

```bash
npm install
npm run dev     # development with hot reload
npm run build   # production build
```

### CLI

```bash
cd social-dl
go run ./cmd/social-dl          # run
make build                      # build binary
go test ./... -v                # tests
```

### Release

```bash
./scripts/release.sh            # auto version: 2026.405.1
./scripts/release.sh patch      # not used, date-based versioning
```

Creates git tag, pushes, GitHub Actions builds extension + CLI for all platforms.

## Tech Stack

- **Extension**: WXT + Preact + Tailwind CSS
- **CLI**: Go + Bubbletea (TUI) + yt-dlp + ffmpeg
- **Build**: GitHub Actions, cross-compilation (5 platforms)
- **Versioning**: Date-based `YYYY.MDD.patch`
