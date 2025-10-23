package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "zero duration",
			duration: 0,
			expected: "0:00",
		},
		{
			name:     "seconds only",
			duration: 45 * time.Second,
			expected: "0:45",
		},
		{
			name:     "minutes and seconds",
			duration: 5*time.Minute + 23*time.Second,
			expected: "5:23",
		},
		{
			name:     "minutes with leading zero seconds",
			duration: 12*time.Minute + 5*time.Second,
			expected: "12:05",
		},
		{
			name:     "exactly one hour",
			duration: 1 * time.Hour,
			expected: "1:00:00",
		},
		{
			name:     "hours, minutes, and seconds",
			duration: 2*time.Hour + 34*time.Minute + 56*time.Second,
			expected: "2:34:56",
		},
		{
			name:     "hours with leading zeros",
			duration: 1*time.Hour + 5*time.Minute + 9*time.Second,
			expected: "1:05:09",
		},
		{
			name:     "typical audiobook chapter",
			duration: 42*time.Minute + 18*time.Second,
			expected: "42:18",
		},
		{
			name:     "long audiobook file",
			duration: 10*time.Hour + 30*time.Minute + 45*time.Second,
			expected: "10:30:45",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestGetMimeType(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{
			name:     "mp3 file",
			filePath: "audio.mp3",
			expected: "audio/mpeg",
		},
		{
			name:     "MP3 uppercase",
			filePath: "AUDIO.MP3",
			expected: "audio/mpeg",
		},
		{
			name:     "m4a file",
			filePath: "audiobook.m4a",
			expected: "audio/mp4",
		},
		{
			name:     "m4b file",
			filePath: "audiobook.m4b",
			expected: "audio/mp4",
		},
		{
			name:     "aac file",
			filePath: "track.aac",
			expected: "audio/aac",
		},
		{
			name:     "flac file",
			filePath: "lossless.flac",
			expected: "audio/flac",
		},
		{
			name:     "ogg file",
			filePath: "podcast.ogg",
			expected: "audio/ogg",
		},
		{
			name:     "unknown extension defaults to mpeg",
			filePath: "audio.xyz",
			expected: "audio/mpeg",
		},
		{
			name:     "file with path",
			filePath: "/path/to/audio.m4a",
			expected: "audio/mp4",
		},
		{
			name:     "file with mixed case extension",
			filePath: "audio.FLaC",
			expected: "audio/flac",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMimeType(tt.filePath)
			if result != tt.expected {
				t.Errorf("getMimeType(%q) = %q, want %q", tt.filePath, result, tt.expected)
			}
		})
	}
}

func TestProcessAudioFile(t *testing.T) {
	baseURL := "https://example.com/audiobooks"
	baseDir := "testdata/audiobook1"
	pubDate := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		filename       string
		expectedTitle  string
		expectedDesc   string
		expectedURL    string
		episodeNum     int
		checkDuration  bool
		minDuration    time.Duration
	}{
		{
			name:          "chapter01 with full metadata",
			filename:      "chapter01.mp3",
			expectedTitle: "Chapter One",
			expectedDesc:  "The beginning of our story",
			expectedURL:   "https://example.com/audiobooks/audiobook1/chapter01.mp3",
			episodeNum:    1,
			checkDuration: true,
			minDuration:   900 * time.Millisecond, // ~1 second
		},
		{
			name:          "chapter02 with metadata",
			filename:      "chapter02.mp3",
			expectedTitle: "Chapter Two",
			expectedDesc:  "The plot thickens",
			expectedURL:   "https://example.com/audiobooks/audiobook1/chapter02.mp3",
			episodeNum:    2,
			checkDuration: true,
			minDuration:   1900 * time.Millisecond, // ~2 seconds
		},
		{
			name:          "chapter03 m4a without comment",
			filename:      "chapter03.m4a",
			expectedTitle: "Chapter Three",
			expectedDesc:  "Chapter Three", // Should fall back to title
			expectedURL:   "https://example.com/audiobooks/audiobook1/chapter03.m4a",
			episodeNum:    3,
			checkDuration: true,
			minDuration:   2900 * time.Millisecond, // ~3 seconds
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(baseDir, tt.filename)

			// Check if test file exists
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Skipf("Test file %s does not exist", filePath)
			}

			episode, err := processAudioFile(filePath, baseURL, baseDir, pubDate, tt.episodeNum)
			if err != nil {
				t.Fatalf("processAudioFile() error = %v", err)
			}

			if episode.Title != tt.expectedTitle {
				t.Errorf("Title = %q, want %q", episode.Title, tt.expectedTitle)
			}

			if episode.Description != tt.expectedDesc {
				t.Errorf("Description = %q, want %q", episode.Description, tt.expectedDesc)
			}

			if episode.URL != tt.expectedURL {
				t.Errorf("URL = %q, want %q", episode.URL, tt.expectedURL)
			}

			if episode.EpisodeNum != tt.episodeNum {
				t.Errorf("EpisodeNum = %d, want %d", episode.EpisodeNum, tt.episodeNum)
			}

			if episode.PubDate != pubDate {
				t.Errorf("PubDate = %v, want %v", episode.PubDate, pubDate)
			}

			if episode.FileSize <= 0 {
				t.Errorf("FileSize = %d, want > 0", episode.FileSize)
			}

			if tt.checkDuration && episode.Duration < tt.minDuration {
				t.Errorf("Duration = %v, want >= %v", episode.Duration, tt.minDuration)
			}
		})
	}
}

func TestScanDirectory(t *testing.T) {
	baseURL := "https://example.com/audiobooks"
	baseDir := "testdata/audiobook1"

	podcast, err := scanDirectory(baseDir, baseURL)
	if err != nil {
		t.Fatalf("scanDirectory() error = %v", err)
	}

	// Check podcast metadata
	if podcast.Title != "audiobook1" {
		t.Errorf("Title = %q, want %q", podcast.Title, "audiobook1")
	}

	expectedDesc := "Audiobook podcast for audiobook1"
	if podcast.Description != expectedDesc {
		t.Errorf("Description = %q, want %q", podcast.Description, expectedDesc)
	}

	// Check cover art URL
	expectedCoverURL := "https://example.com/audiobooks/audiobook1/cover.jpg"
	if podcast.CoverArtURL != expectedCoverURL {
		t.Errorf("CoverArtURL = %q, want %q", podcast.CoverArtURL, expectedCoverURL)
	}

	// Check episode count
	if len(podcast.Episodes) != 3 {
		t.Fatalf("len(Episodes) = %d, want 3", len(podcast.Episodes))
	}

	// Check episodes are sorted alphabetically
	expectedTitles := []string{"Chapter One", "Chapter Two", "Chapter Three"}
	for i, ep := range podcast.Episodes {
		if ep.Title != expectedTitles[i] {
			t.Errorf("Episode[%d].Title = %q, want %q", i, ep.Title, expectedTitles[i])
		}
	}

	// Check episode numbers are sequential
	for i, ep := range podcast.Episodes {
		expectedNum := i + 1
		if ep.EpisodeNum != expectedNum {
			t.Errorf("Episode[%d].EpisodeNum = %d, want %d", i, ep.EpisodeNum, expectedNum)
		}
	}

	// Check pubDates are sequential (1 second apart)
	for i := 1; i < len(podcast.Episodes); i++ {
		diff := podcast.Episodes[i].PubDate.Sub(podcast.Episodes[i-1].PubDate)
		if diff != time.Second {
			t.Errorf("Episode[%d] pubDate diff = %v, want 1s", i, diff)
		}
	}
}

// normalizeRSS removes timestamps from RSS feed for comparison
func normalizeRSS(rss string) string {
	// Remove lastBuildDate (changes every time)
	rss = regexp.MustCompile(`<lastBuildDate>.*?</lastBuildDate>`).ReplaceAllString(rss, "<lastBuildDate>NORMALIZED</lastBuildDate>")
	// Remove pubDate (changes every time)
	rss = regexp.MustCompile(`<pubDate>.*?</pubDate>`).ReplaceAllString(rss, "<pubDate>NORMALIZED</pubDate>")
	return rss
}

func TestGenerateRSSGolden(t *testing.T) {
	baseURL := "https://example.com/audiobooks"
	baseDir := "testdata/audiobook1"

	// Scan directory
	podcast, err := scanDirectory(baseDir, baseURL)
	if err != nil {
		t.Fatalf("scanDirectory() error = %v", err)
	}

	// Generate RSS
	rss := generateRSS(podcast)

	// Read golden file
	goldenPath := filepath.Join(baseDir, "golden.rss")
	goldenBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("Failed to read golden file: %v\nRun ./generate_test_fixtures.sh to create it", err)
	}
	golden := string(goldenBytes)

	// Normalize both (remove timestamps)
	normalizedRSS := normalizeRSS(rss)
	normalizedGolden := normalizeRSS(golden)

	// Compare
	if normalizedRSS != normalizedGolden {
		t.Errorf("Generated RSS does not match golden file.\n\nGenerated:\n%s\n\nGolden:\n%s\n\nIf the change is intentional, run ./generate_test_fixtures.sh to update the golden file.", normalizedRSS, normalizedGolden)
	}
}
