# 📚🎧 bookast

A simple podcast feed generator for audiobooks. Generates RSS feeds from directories containing audio files.

## Requirements

- Go 1.19+
- ffmpeg

## Installation

```bash
go build -o bookast
```

## Usage

```bash
./bookast --base-url https://your-server.com/audiobooks /path/to/audiobook-directory
```

Generates `podcast.rss` in the specified directory.