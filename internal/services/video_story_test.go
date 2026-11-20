package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseYouTubeVideoID(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantID  string
		wantErr bool
	}{
		{name: "watch URL", url: "https://www.youtube.com/watch?v=abcdefghijk", wantID: "abcdefghijk"},
		{name: "short URL", url: "https://youtu.be/abcdefghijk?t=30", wantID: "abcdefghijk"},
		{name: "shorts URL", url: "https://youtube.com/shorts/abcdefghijk", wantID: "abcdefghijk"},
		{name: "embed URL", url: "https://youtube.com/embed/abcdefghijk", wantID: "abcdefghijk"},
		{name: "lookalike host", url: "https://youtube.com.example.org/watch?v=abcdefghijk", wantErr: true},
		{name: "untrusted host", url: "http://127.0.0.1/watch?v=abcdefghijk", wantErr: true},
		{name: "invalid ID", url: "https://www.youtube.com/watch?v=short", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseYouTubeVideoID(test.url)
			if test.wantErr {
				if !errors.Is(err, ErrInvalidYouTubeURL) {
					t.Fatalf("expected invalid URL error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseYouTubeVideoID returned an error: %v", err)
			}
			if got != test.wantID {
				t.Fatalf("got video ID %q, want %q", got, test.wantID)
			}
		})
	}
}

func TestYouTubeTranscriptFetcherFetchesEnglishCaptions(t *testing.T) {
	playerResponse, err := json.Marshal(map[string]interface{}{
		"videoDetails": map[string]string{"title": "Ancient Humans"},
		"captions": map[string]interface{}{
			"playerCaptionsTracklistRenderer": map[string]interface{}{
				"captionTracks": []map[string]string{{
					"baseUrl":      "https://www.youtube.com/api/timedtext?v=abcdefghijk",
					"languageCode": "en",
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/watch" {
			body := "var ytInitialPlayerResponse = " + string(playerResponse) + ";"
			return testHTTPResponse(http.StatusOK, body), nil
		}
		if request.URL.Path == "/api/timedtext" && request.URL.Query().Get("fmt") == "json3" {
			return testHTTPResponse(http.StatusOK, `{"events":[{"segs":[{"utf8":"People gathered "},{"utf8":"around the fire."}]}]}`), nil
		}
		return nil, fmt.Errorf("unexpected request URL %s", request.URL)
	})}
	fetcher := &youtubeTranscriptFetcher{client: client}

	title, language, transcript, err := fetcher.Fetch(context.Background(), "abcdefghijk")
	if err != nil {
		t.Fatalf("Fetch returned an error: %v", err)
	}
	if title != "Ancient Humans" || language != "en" || transcript != "People gathered around the fire." {
		t.Fatalf("unexpected fetch result: title=%q language=%q transcript=%q", title, language, transcript)
	}
}

func TestOpenAIStoryGeneratorUsesConfiguredChatAPI(t *testing.T) {
	story := VideoStory{
		Summary: "The video explains how people found food and shared work.",
		Scenes: []VideoScene{{
			SceneNumber:  1,
			Visual:       "A group gathers near a fire.",
			Narration:    "Daily life depended on cooperation.",
			OnScreenText: "Life was a team effort",
		}},
	}
	storyJSON, err := json.Marshal(story)
	if err != nil {
		t.Fatal(err)
	}
	completion, err := json.Marshal(map[string]interface{}{
		"choices": []map[string]interface{}{{
			"message": map[string]string{"content": string(storyJSON)},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing API authorization header")
		}
		body, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			t.Errorf("reading request body: %v", readErr)
		}
		if !strings.Contains(string(body), "People shared daily work") {
			t.Errorf("request did not include source transcript")
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(completion)
	}))
	defer server.Close()

	generator := &openAIStoryGenerator{
		client:  server.Client(),
		apiKey:  "test-key",
		baseURL: server.URL + "/v1",
		model:   "test-model",
	}
	result, err := generator.Generate(
		context.Background(),
		VideoStoryRequest{Audience: "general audience", Tone: "curious"},
		"Daily life long ago",
		"en",
		"People shared daily work.",
	)
	if err != nil {
		t.Fatalf("Generate returned an error: %v", err)
	}
	if result.Summary != story.Summary || len(result.Scenes) != 1 {
		t.Fatalf("unexpected generated story: %#v", result)
	}
}

func TestOpenAIStoryGeneratorRequiresAPIKey(t *testing.T) {
	generator := &openAIStoryGenerator{}
	_, err := generator.Generate(context.Background(), VideoStoryRequest{}, "", "", "")
	if !errors.Is(err, ErrStoryProviderNotConfigured) {
		t.Fatalf("expected provider configuration error, got %v", err)
	}
}

func TestVideoStoryServiceSetsSourceAndPreferenceDefaults(t *testing.T) {
	generator := &stubStoryGenerator{}
	service := &videoStoryService{
		transcripts: stubTranscriptFetcher{
			title:      "Source title",
			language:   "en",
			transcript: "Source transcript",
		},
		generator: generator,
	}

	result, err := service.Create(context.Background(), VideoStoryRequest{
		VideoURL: "https://youtu.be/abcdefghijk",
	})
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	if result.VideoURL != "https://www.youtube.com/watch?v=abcdefghijk" || result.VideoTitle != "Source title" {
		t.Fatalf("source metadata was not attached: %#v", result)
	}
	if generator.request.Audience != "general audience" || generator.request.Tone != "curious and cinematic" {
		t.Fatalf("unexpected default preferences: %#v", generator.request)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func testHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

type stubTranscriptFetcher struct {
	title, language, transcript string
}

func (f stubTranscriptFetcher) Fetch(context.Context, string) (string, string, string, error) {
	return f.title, f.language, f.transcript, nil
}

type stubStoryGenerator struct {
	request VideoStoryRequest
}

func (g *stubStoryGenerator) CheckConfiguration() error {
	return nil
}

func (g *stubStoryGenerator) Generate(
	_ context.Context,
	request VideoStoryRequest,
	_, _, _ string,
) (*VideoStory, error) {
	g.request = request
	return &VideoStory{Summary: "summary", Scenes: []VideoScene{{SceneNumber: 1}}}, nil
}
