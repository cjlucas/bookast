package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dhowden/tag"
)

type Episode struct {
	Title       string
	Description string
	FilePath    string
	Duration    time.Duration
	FileSize    int64
	PubDate     time.Time
	URL         string
}

type Podcast struct {
	Title       string
	Description string
	Episodes    []Episode
}

func main() {
	var baseURL string
	flag.StringVar(&baseURL, "base-url", "", "Base URL for hosting the files (required)")
	flag.Parse()

	if baseURL == "" {
		fmt.Fprintf(os.Stderr, "Error: --base-url is required\n")
		os.Exit(1)
	}

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s --base-url <url> <directory>\n", os.Args[0])
		os.Exit(1)
	}

	directory := flag.Arg(0)
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Directory '%s' does not exist\n", directory)
		os.Exit(1)
	}

	podcast, err := scanDirectory(directory, baseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning directory: %v\n", err)
		os.Exit(1)
	}

	if len(podcast.Episodes) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No audio files found in directory '%s'\n", directory)
		os.Exit(1)
	}

	rssContent := generateRSS(podcast)
	rssFile := filepath.Join(directory, "podcast.rss")

	err = os.WriteFile(rssFile, []byte(rssContent), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing RSS file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated RSS feed: %s\n", rssFile)
	fmt.Printf("Found %d episodes\n", len(podcast.Episodes))
}

func scanDirectory(dir string, baseURL string) (*Podcast, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	podcast := &Podcast{
		Title:       filepath.Base(dir),
		Description: fmt.Sprintf("Audiobook podcast for %s", filepath.Base(dir)),
		Episodes:    []Episode{},
	}

	var audioFiles []string
	supportedExts := map[string]bool{
		".mp3":  true,
		".m4a":  true,
		".m4b":  true,
		".aac":  true,
		".flac": true,
		".ogg":  true,
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if supportedExts[ext] {
			audioFiles = append(audioFiles, entry.Name())
		}
	}

	sort.Strings(audioFiles)

	for _, filename := range audioFiles {
		fullPath := filepath.Join(dir, filename)
		episode, err := processAudioFile(fullPath, baseURL, dir)
		if err != nil {
			return nil, fmt.Errorf("failed to process %s: %v", filename, err)
		}
		podcast.Episodes = append(podcast.Episodes, *episode)
	}

	return podcast, nil
}

func processAudioFile(filePath string, baseURL string, baseDir string) (*Episode, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	metadata, err := tag.ReadFrom(file)
	if err != nil {
		return nil, err
	}

	filename := filepath.Base(filePath)
	dirName := filepath.Base(baseDir)

	escapedDir := url.PathEscape(dirName)
	escapedFile := url.PathEscape(filename)
	fileURL := strings.TrimSuffix(baseURL, "/") + "/" + escapedDir + "/" + escapedFile

	title := metadata.Title()
	if title == "" {
		title = strings.TrimSuffix(filename, filepath.Ext(filename))
	}

	description := ""
	if metadata.Comment() != "" {
		description = metadata.Comment()
	} else {
		description = title
	}

	episode := &Episode{
		Title:       title,
		Description: description,
		FilePath:    filePath,
		FileSize:    fileInfo.Size(),
		PubDate:     fileInfo.ModTime(),
		URL:         fileURL,
	}

	return episode, nil
}

func generateRSS(podcast *Podcast) string {
	var sb strings.Builder

	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString("\n")
	sb.WriteString(`<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">`)
	sb.WriteString("\n<channel>\n")

	sb.WriteString(fmt.Sprintf("  <title>%s</title>\n", escapeXML(podcast.Title)))
	sb.WriteString(fmt.Sprintf("  <description>%s</description>\n", escapeXML(podcast.Description)))
	sb.WriteString("  <language>en-us</language>\n")
	sb.WriteString(fmt.Sprintf("  <lastBuildDate>%s</lastBuildDate>\n", time.Now().Format(time.RFC1123Z)))

	for _, episode := range podcast.Episodes {
		sb.WriteString("  <item>\n")
		sb.WriteString(fmt.Sprintf("    <title>%s</title>\n", escapeXML(episode.Title)))
		sb.WriteString(fmt.Sprintf("    <description>%s</description>\n", escapeXML(episode.Description)))
		sb.WriteString(fmt.Sprintf("    <pubDate>%s</pubDate>\n", episode.PubDate.Format(time.RFC1123Z)))
		sb.WriteString(fmt.Sprintf("    <enclosure url=\"%s\" length=\"%d\" type=\"%s\" />\n",
			escapeXML(episode.URL), episode.FileSize, getMimeType(episode.FilePath)))
		sb.WriteString(fmt.Sprintf("    <guid>%s</guid>\n", escapeXML(episode.URL)))
		sb.WriteString("  </item>\n")
	}

	sb.WriteString("</channel>\n")
	sb.WriteString("</rss>\n")

	return sb.String()
}

func getMimeType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".m4a", ".m4b":
		return "audio/mp4"
	case ".aac":
		return "audio/aac"
	case ".flac":
		return "audio/flac"
	case ".ogg":
		return "audio/ogg"
	default:
		return "audio/mpeg"
	}
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}