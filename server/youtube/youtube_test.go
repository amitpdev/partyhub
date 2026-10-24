package youtube

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type runnerFunc func(context.Context, string, ...string) ([]byte, error)

func (run runnerFunc) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return run(ctx, name, args...)
}

func TestParseURL(t *testing.T) {
	tests := map[string]string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":   "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ?t=42":             "dQw4w9WgXcQ",
		"https://youtube.com/shorts/dQw4w9WgXcQ":        "dQw4w9WgXcQ",
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ": "dQw4w9WgXcQ",
		"https://www.youtube.com/embed/dQw4w9WgXcQ":     "dQw4w9WgXcQ",
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			id, canonical, err := ParseURL(input)
			if err != nil {
				t.Fatal(err)
			}
			if id != want {
				t.Fatalf("id = %q, want %q", id, want)
			}
			if canonical != "https://www.youtube.com/watch?v="+want {
				t.Fatalf("canonical = %q", canonical)
			}
		})
	}
}

func TestParseURLRejectsUnsupportedInput(t *testing.T) {
	for _, input := range []string{
		"",
		"https://spotify.com/track/abc",
		"https://youtube.com/watch?v=short",
		"https://youtube.com/watch?v=dQw4w9WgXcQ&list=PL123",
		"javascript:alert(1)",
	} {
		if _, _, err := ParseURL(input); err == nil {
			t.Fatalf("ParseURL(%q) returned no error", input)
		}
	}
}

func TestSearchFiltersAndNormalizesResults(t *testing.T) {
	output := strings.Join([]string{
		`{"id":"aaaaaaaaaaa","title":"First song","channel":"Channel A","thumbnail":"https://img.test/a.jpg","duration":180,"url":"aaaaaaaaaaa","live_status":"not_live"}`,
		`{"id":"bbbbbbbbbbb","title":"Live","channel":"Channel B","duration":200,"is_live":true}`,
		`{"id":"ccccccccccc","title":"Upcoming","channel":"Channel C","duration":200,"live_status":"is_upcoming"}`,
		`{"id":"ddddddddddd","title":"Too long","channel":"Channel D","duration":901}`,
		`{"id":"eeeeeeeeeee","title":"Unknown duration","channel":"Channel E","duration":0}`,
		`{"id":"fffffffffff","title":"Short","channel":"Channel F","duration":40,"webpage_url":"https://www.youtube.com/shorts/fffffffffff"}`,
		`{"id":"not-valid","title":"Bad id","channel":"Channel G","duration":200}`,
		`{"id":"aaaaaaaaaaa","title":"Duplicate","channel":"Channel A","duration":180}`,
		`{"id":"ggggggggggg","title":"","uploader":"Uploader G","duration":900,"thumbnails":[{"url":"https://img.test/small.jpg"},{"url":"https://img.test/large.jpg"}]}`,
	}, "\n")
	resolver := Resolver{Runner: runnerFunc(func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "yt-dlp" {
			t.Fatalf("name = %q", name)
		}
		wantLast := "ytsearch60:dance floor"
		if args[len(args)-1] != wantLast {
			t.Fatalf("last argument = %q, want %q", args[len(args)-1], wantLast)
		}
		return []byte(output), nil
	})}

	results, err := resolver.Search(context.Background(), "  dance floor  ")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if got := results[0]; got.ProviderID != "aaaaaaaaaaa" ||
		got.URL != "https://www.youtube.com/watch?v=aaaaaaaaaaa" ||
		got.Source != "youtube" {
		t.Fatalf("unexpected first result: %#v", got)
	}
	if got := results[1]; got.Title != "Untitled video" ||
		got.Channel != "Uploader G" ||
		got.Thumbnail != "https://img.test/large.jpg" {
		t.Fatalf("unexpected fallback metadata: %#v", got)
	}
}

func TestSearchCapsEligibleResults(t *testing.T) {
	var output strings.Builder
	for index := 0; index < SearchResultLimit+3; index++ {
		id := fmt.Sprintf("%011d", index)
		fmt.Fprintf(&output, `{"id":%q,"title":%q,"duration":120}`+"\n", id, fmt.Sprintf("Song %d", index))
	}
	resolver := Resolver{Runner: runnerFunc(func(context.Context, string, ...string) ([]byte, error) {
		return []byte(output.String()), nil
	})}

	results, err := resolver.Search(context.Background(), "party")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != SearchResultLimit {
		t.Fatalf("len(results) = %d, want %d", len(results), SearchResultLimit)
	}
}

func TestSearchValidatesQuery(t *testing.T) {
	resolver := Resolver{Runner: runnerFunc(func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("runner should not be called")
		return nil, nil
	})}
	if _, err := resolver.Search(context.Background(), "   "); !errors.Is(err, ErrEmptySearchQuery) {
		t.Fatalf("empty query error = %v", err)
	}
	if _, err := resolver.Search(context.Background(), strings.Repeat("x", maxSearchQueryLength+1)); !errors.Is(err, ErrSearchQueryTooLong) {
		t.Fatalf("long query error = %v", err)
	}
}

func TestSearchReportsUnreadableResults(t *testing.T) {
	resolver := Resolver{Runner: runnerFunc(func(context.Context, string, ...string) ([]byte, error) {
		return []byte("{not json}\n"), nil
	})}
	if _, err := resolver.Search(context.Background(), "party"); err == nil ||
		err.Error() != "YouTube returned unreadable search results" {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchClassifiesCommandFailures(t *testing.T) {
	resolver := Resolver{Runner: runnerFunc(func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("command failed")
	})}
	if _, err := resolver.Search(context.Background(), "party"); !errors.Is(err, ErrSearchUnavailable) {
		t.Fatalf("error = %v", err)
	}
}

func TestSearchHonorsContextCancellation(t *testing.T) {
	resolver := Resolver{Runner: runnerFunc(func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := resolver.Search(ctx, "party"); !errors.Is(err, ErrSearchUnavailable) {
		t.Fatalf("error = %v, want unavailable for caller cancellation", err)
	}
}
