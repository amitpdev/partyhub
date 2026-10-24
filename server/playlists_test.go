package main

import (
	"net/http"
	"net/http/httptest"
	"partyhub/server/party"
	"partyhub/server/player"
	"partyhub/server/youtube"
	"strings"
	"testing"
)

func TestPlaylistHTTPAuthorizationAndValidation(t *testing.T) {
	manager := party.New(player.New(), youtube.Resolver{}, "")
	mux := http.NewServeMux()
	registerPlaylists(mux, manager)
	for _, tc := range []struct {
		method, host, body string
		status             int
	}{
		{"GET", "localhost", "", 200},
		{"GET", "party.example", "", 403},
		{"POST", "party.example", `{"action":"delete","id":"anything"}`, 403},
		{"POST", "localhost", `{"action":"import","backup":{"version":99,"playlists":[]}}`, 400},
		{"POST", "localhost", `{"action":"save","name":"Empty"}`, 400},
		{"POST", "localhost", `{"action":"import","backup":{"version":1,"playlists":[]}}`, 200},
		{"POST", "localhost", `{"action":"import","backup":{"version":1,"playlists":[]}} {}`, 400},
	} {
		request := httptest.NewRequest(tc.method, "http://"+tc.host+"/api/playlists", strings.NewReader(tc.body))
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Errorf("%s %s %s: %d %s", tc.method, tc.host, tc.body, response.Code, response.Body.String())
		}
	}
}
