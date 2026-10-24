package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	SearchResultLimit    = 24
	searchCandidateLimit = 60
	maxSearchQueryLength = 200
	MaxTrackSeconds      = 15 * 60
	youtubeTimeout       = 20 * time.Second
)

var (
	videoIDPattern        = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	ErrEmptySearchQuery   = errors.New("enter something to search for")
	ErrSearchQueryTooLong = errors.New("keep the search under 200 characters")
	ErrSearchTimedOut     = errors.New("YouTube search took too long; try a more specific search")
	ErrSearchUnavailable  = errors.New("YouTube search is temporarily unavailable")
	ErrUnreadableResults  = errors.New("YouTube returned unreadable search results")
)

type Track struct {
	Source     string  `json:"source"`
	ProviderID string  `json:"providerId"`
	URL        string  `json:"url"`
	Title      string  `json:"title"`
	Channel    string  `json:"channel"`
	Thumbnail  string  `json:"thumbnail"`
	Duration   float64 `json:"duration"`
}

type CommandRunner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

type Resolver struct {
	Runner CommandRunner
}

func (resolver Resolver) Resolve(rawURL string) (Track, error) {
	id, canonical, err := ParseURL(rawURL)
	if err != nil {
		return Track{}, err
	}
	runner := resolver.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), youtubeTimeout)
	defer cancel()
	output, err := runner.Output(
		ctx,
		"yt-dlp",
		"--dump-single-json",
		"--skip-download",
		"--no-playlist",
		"--no-warnings",
		"--",
		canonical,
	)
	if err != nil {
		return Track{}, fmt.Errorf("could not read this YouTube video: %w", err)
	}
	var item struct {
		ID           string  `json:"id"`
		Title        string  `json:"title"`
		Channel      string  `json:"channel"`
		Uploader     string  `json:"uploader"`
		Thumbnail    string  `json:"thumbnail"`
		Duration     float64 `json:"duration"`
		Availability string  `json:"availability"`
		IsLive       bool    `json:"is_live"`
		LiveStatus   string  `json:"live_status"`
	}
	if err := json.Unmarshal(output, &item); err != nil {
		return Track{}, errors.New("YouTube returned unreadable track details")
	}
	if item.ID == "" {
		item.ID = id
	}
	if item.Title == "" {
		item.Title = "Untitled video"
	}
	if item.Channel == "" {
		item.Channel = item.Uploader
	}
	if item.Channel == "" {
		item.Channel = "Unknown channel"
	}
	if item.IsLive || item.LiveStatus == "is_live" || item.LiveStatus == "is_upcoming" {
		return Track{}, errors.New("live streams are not supported")
	}
	return Track{
		Source:     "youtube",
		ProviderID: item.ID,
		URL:        canonical,
		Title:      item.Title,
		Channel:    item.Channel,
		Thumbnail:  item.Thumbnail,
		Duration:   item.Duration,
	}, nil
}

func (resolver Resolver) Search(ctx context.Context, query string) ([]Track, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrEmptySearchQuery
	}
	if len([]rune(query)) > maxSearchQueryLength {
		return nil, ErrSearchQueryTooLong
	}

	runner := resolver.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	searchContext, cancel := context.WithTimeout(ctx, youtubeTimeout)
	defer cancel()
	output, err := runner.Output(
		searchContext,
		"yt-dlp",
		"--flat-playlist",
		"--dump-json",
		"--no-warnings",
		"--",
		fmt.Sprintf("ytsearch%d:%s", searchCandidateLimit, query),
	)
	if err != nil {
		if errors.Is(searchContext.Err(), context.DeadlineExceeded) ||
			errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrSearchTimedOut
		}
		return nil, fmt.Errorf("%w: %v", ErrSearchUnavailable, err)
	}

	results, err := decodeSearchResults(output)
	if err != nil {
		return nil, err
	}
	return results, nil
}

type searchItem struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Channel    string `json:"channel"`
	Uploader   string `json:"uploader"`
	Thumbnail  string `json:"thumbnail"`
	Thumbnails []struct {
		URL string `json:"url"`
	} `json:"thumbnails"`
	Duration    float64 `json:"duration"`
	IsLive      bool    `json:"is_live"`
	LiveStatus  string  `json:"live_status"`
	URL         string  `json:"url"`
	WebpageURL  string  `json:"webpage_url"`
	OriginalURL string  `json:"original_url"`
}

func decodeSearchResults(output []byte) ([]Track, error) {
	decoder := json.NewDecoder(bytes.NewReader(output))
	tracks := make([]Track, 0, SearchResultLimit)
	seen := make(map[string]struct{})
	for len(tracks) < SearchResultLimit {
		var item searchItem
		if err := decoder.Decode(&item); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, ErrUnreadableResults
		}
		if !eligibleSearchItem(item) {
			continue
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		channel := strings.TrimSpace(item.Channel)
		if channel == "" {
			channel = strings.TrimSpace(item.Uploader)
		}
		if channel == "" {
			channel = "Unknown channel"
		}
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = "Untitled video"
		}
		thumbnail := item.Thumbnail
		if thumbnail == "" {
			for index := len(item.Thumbnails) - 1; index >= 0; index-- {
				if item.Thumbnails[index].URL != "" {
					thumbnail = item.Thumbnails[index].URL
					break
				}
			}
		}
		tracks = append(tracks, Track{
			Source:     "youtube",
			ProviderID: item.ID,
			URL:        "https://www.youtube.com/watch?v=" + item.ID,
			Title:      title,
			Channel:    channel,
			Thumbnail:  thumbnail,
			Duration:   item.Duration,
		})
	}
	return tracks, nil
}

func eligibleSearchItem(item searchItem) bool {
	if !videoIDPattern.MatchString(item.ID) || item.Duration <= 0 || item.Duration > MaxTrackSeconds {
		return false
	}
	if item.IsLive || item.LiveStatus == "is_live" || item.LiveStatus == "is_upcoming" {
		return false
	}
	for _, candidateURL := range []string{item.URL, item.WebpageURL, item.OriginalURL} {
		if strings.Contains(strings.ToLower(candidateURL), "/shorts/") {
			return false
		}
	}
	return true
}

func ParseURL(rawURL string) (string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", "", errors.New("enter a complete YouTube link")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", "", errors.New("only http and https YouTube links are supported")
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	host = strings.TrimPrefix(host, "m.")
	if parsed.Query().Get("list") != "" {
		return "", "", errors.New("playlist links are not supported; add one video at a time")
	}
	var id string
	switch host {
	case "youtu.be":
		id = strings.Trim(parsed.Path, "/")
		if strings.Contains(id, "/") {
			id = strings.Split(id, "/")[0]
		}
	case "youtube.com", "music.youtube.com":
		if parsed.Path == "/watch" {
			id = parsed.Query().Get("v")
		} else {
			parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
			if len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live") {
				id = parts[1]
			}
		}
	default:
		return "", "", errors.New("only YouTube links are supported in phase 1")
	}
	if !videoIDPattern.MatchString(id) {
		return "", "", errors.New("this does not look like a valid YouTube video link")
	}
	return id, "https://www.youtube.com/watch?v=" + id, nil
}
