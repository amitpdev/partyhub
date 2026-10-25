package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type ModelChoice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Service) ChatGPTModels(ctx context.Context, id string) ([]ModelChoice, error) {
	token, err := s.chatGPT.access(ctx, id)
	if err != nil {
		return nil, err
	}
	raw, err := s.chatGPT.http(ctx, "GET", apiResource+"/models", nil, token)
	if err != nil {
		return nil, err
	}
	var result struct {
		Models []struct {
			Slug       string `json:"slug"`
			Name       string `json:"display_name"`
			Visibility string `json:"visibility"`
		} `json:"models"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Models == nil {
		return nil, errors.New("Could not load your ChatGPT models. Try again.")
	}
	models := []ModelChoice{}
	seen := map[string]bool{}
	for _, m := range result.Models {
		if m.Visibility == "list" && m.Slug != "" && !seen[m.Slug] {
			name := m.Name
			if name == "" {
				name = m.Slug
			}
			models = append(models, ModelChoice{m.Slug, name})
			seen[m.Slug] = true
		}
	}
	return models, nil
}
func (s *Service) completeChatGPT(ctx context.Context, d Draft, instructions, input string) (string, error) {
	ctx, release, err := s.chatGPT.requestContext(ctx, d.AccountID)
	if err != nil {
		return "", err
	}
	defer release()
	token, err := s.chatGPT.access(ctx, d.AccountID)
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(map[string]any{"model": d.Model, "instructions": instructions, "input": []map[string]string{{"role": "user", "content": input}}, "store": false, "stream": true})
	req, err := http.NewRequestWithContext(ctx, "POST", apiResource+"/responses", bytes.NewReader(body))
	if err != nil {
		return "", errors.New("Could not prepare ChatGPT request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	client := *s.Client
	client.Timeout = 40 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Do(req)
	if err != nil {
		return "", errors.New("Could not reach ChatGPT. Try again.")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if res.StatusCode == 401 {
			return "", errors.New("ChatGPT session expired. Sign in again in AI settings.")
		}
		if res.StatusCode == 429 {
			return "", errors.New("ChatGPT usage limit reached. Use Manage usage in AI settings.")
		}
		if res.StatusCode == 403 {
			return "", errors.New("ChatGPT plan access is unavailable. Check your account permissions.")
		}
		return "", errors.New("ChatGPT could not complete the pick. Check the model and try again.")
	}
	return readChatGPTStream(res.Body)
}
func readChatGPTStream(body io.Reader) (string, error) {
	scanner := bufio.NewScanner(io.LimitReader(body, 2*1024*1024+1))
	scanner.Buffer(make([]byte, 4096), 512*1024)
	var text strings.Builder
	data := []string{}
	completed := false
	process := func() error {
		if len(data) == 0 {
			return nil
		}
		raw := strings.Join(data, "\n")
		data = nil
		if raw == "[DONE]" {
			return nil
		}
		var event struct {
			Type  string `json:"type"`
			Delta string `json:"delta"`
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
			Response struct {
				Status string `json:"status"`
				Error  struct {
					Code string `json:"code"`
				} `json:"error"`
				Output []struct {
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"output"`
			} `json:"response"`
		}
		if json.Unmarshal([]byte(raw), &event) != nil {
			return errors.New("ChatGPT returned an unreadable stream. Try again.")
		}
		switch event.Type {
		case "response.output_text.delta":
			text.WriteString(event.Delta)
		case "response.completed":
			if event.Response.Status != "completed" {
				return errors.New("ChatGPT did not finish the response. Try again.")
			}
			completed = true
			if text.Len() == 0 {
				for _, o := range event.Response.Output {
					for _, p := range o.Content {
						if p.Type == "output_text" {
							text.WriteString(p.Text)
						}
					}
				}
			}
		case "response.failed", "error":
			code := event.Response.Error.Code
			if code == "" {
				code = event.Error.Code
			}
			if strings.HasPrefix(code, "subscription_sharing_usage_") {
				return errors.New("ChatGPT usage limit reached or unavailable. Use Manage usage in AI settings.")
			}
			return errors.New("ChatGPT could not complete the response. Try again.")
		case "response.incomplete":
			return errors.New("ChatGPT response was incomplete. Try again.")
		}
		if text.Len() > 256*1024 {
			return errors.New("ChatGPT response was too large. Try another model.")
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := process(); err != nil {
				return "", err
			}
			if completed {
				break
			}
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return "", errors.New("ChatGPT connection was interrupted. Try again.")
	}
	if !completed {
		if err := process(); err != nil {
			return "", err
		}
	}
	if !completed || strings.TrimSpace(text.String()) == "" {
		return "", errors.New("ChatGPT connection ended before completion. Try again.")
	}
	return text.String(), nil
}
