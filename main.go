package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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
	EpisodeNum  int
}

type Podcast struct {
	Title        string
	Description  string
	Episodes     []Episode
	CoverArtURL  string
}

// RSS XML structures
type RSS struct {
	XMLName  xml.Name `xml:"rss"`
	Version  string   `xml:"version,attr"`
	ITunesNS string   `xml:"xmlns:itunes,attr"`
	Channel  *Channel `xml:"channel"`
}

type Channel struct {
	Title         string        `xml:"title"`
	Description   string        `xml:"description"`
	Language      string        `xml:"language"`
	ItunesType    string        `xml:"itunes:type"`
	ItunesImage   *ItunesImage  `xml:"itunes:image,omitempty"`
	LastBuildDate string        `xml:"lastBuildDate"`
	Items         []Item        `xml:"item"`
}

type ItunesImage struct {
	Href string `xml:"href,attr"`
}

type Item struct {
	Title          string     `xml:"title"`
	Description    string     `xml:"description"`
	PubDate        string     `xml:"pubDate"`
	ItunesEpisode  int        `xml:"itunes:episode"`
	ItunesDuration string     `xml:"itunes:duration,omitempty"`
	Enclosure      *Enclosure `xml:"enclosure"`
	GUID           string     `xml:"guid"`
}

type Enclosure struct {
	URL    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
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
	var coverArtFile string
	supportedAudioExts := map[string]bool{
		".mp3":  true,
		".m4a":  true,
		".m4b":  true,
		".aac":  true,
		".flac": true,
		".ogg":  true,
	}
	supportedImageExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if supportedAudioExts[ext] {
			audioFiles = append(audioFiles, entry.Name())
		} else if supportedImageExts[ext] && coverArtFile == "" {
			coverArtFile = entry.Name()
		}
	}

	sort.Strings(audioFiles)

	now := time.Now()
	for i, filename := range audioFiles {
		fullPath := filepath.Join(dir, filename)
		episode, err := processAudioFile(fullPath, baseURL, dir, now.Add(time.Duration(i)*time.Second), i+1)
		if err != nil {
			return nil, fmt.Errorf("failed to process %s: %v", filename, err)
		}
		podcast.Episodes = append(podcast.Episodes, *episode)
	}

	// Set cover art URL if image file found
	if coverArtFile != "" {
		dirName := filepath.Base(dir)
		escapedDir := url.PathEscape(dirName)
		escapedFile := url.PathEscape(coverArtFile)
		podcast.CoverArtURL = strings.TrimSuffix(baseURL, "/") + "/" + escapedDir + "/" + escapedFile
	}

	return podcast, nil
}

func getDurationWithFFmpeg(filePath string) (time.Duration, error) {
	cmd := exec.Command("ffprobe", "-v", "quiet", "-show_entries", "format=duration", "-of", "csv=p=0", filePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %v", err)
	}

	durationStr := strings.TrimSpace(string(output))
	if durationStr == "" {
		return 0, fmt.Errorf("no duration found in ffprobe output")
	}

	durationSeconds, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %v", err)
	}

	return time.Duration(durationSeconds * float64(time.Second)), nil
}

func processAudioFile(filePath string, baseURL string, baseDir string, pubDate time.Time, episodeNum int) (*Episode, error) {
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
	comment := metadata.Comment()
	if comment != "" && comment != "iTunPGAP" {
		description = comment
	} else {
		description = title
	}

	duration, err := getDurationWithFFmpeg(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get duration: %v", err)
	}

	episode := &Episode{
		Title:       title,
		Description: description,
		FilePath:    filePath,
		Duration:    duration,
		FileSize:    fileInfo.Size(),
		PubDate:     pubDate,
		URL:         fileURL,
		EpisodeNum:  episodeNum,
	}

	return episode, nil
}

func generateRSS(podcast *Podcast) string {
	// Build items
	items := make([]Item, 0, len(podcast.Episodes))
	for _, ep := range podcast.Episodes {
		item := Item{
			Title:         ep.Title,
			Description:   ep.Description,
			PubDate:       ep.PubDate.Format(time.RFC1123Z),
			ItunesEpisode: ep.EpisodeNum,
			Enclosure: &Enclosure{
				URL:    ep.URL,
				Length: ep.FileSize,
				Type:   getMimeType(ep.FilePath),
			},
			GUID: ep.URL,
		}

		if ep.Duration > 0 {
			item.ItunesDuration = formatDuration(ep.Duration)
		}

		items = append(items, item)
	}

	// Build channel
	channel := &Channel{
		Title:         podcast.Title,
		Description:   podcast.Description,
		Language:      "en-us",
		ItunesType:    "serial",
		LastBuildDate: time.Now().Format(time.RFC1123Z),
		Items:         items,
	}

	if podcast.CoverArtURL != "" {
		channel.ItunesImage = &ItunesImage{
			Href: podcast.CoverArtURL,
		}
	}

	// Build RSS
	rss := &RSS{
		Version:  "2.0",
		ITunesNS: "http://www.itunes.com/dtds/podcast-1.0.dtd",
		Channel:  channel,
	}

	// Marshal to XML
	output, err := xml.MarshalIndent(rss, "", "  ")
	if err != nil {
		return ""
	}

	return xml.Header + string(output) + "\n"
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

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}