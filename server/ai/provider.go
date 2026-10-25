package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

func (s *Service) request(ctx context.Context, d Draft, method, path string, body any) ([]byte, error) {
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, errors.New("could not encode AI request")
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, d.BaseURL+path, payload)
	if err != nil {
		return nil, errors.New("invalid AI server URL")
	}
	req.Header.Set("Content-Type", "application/json")
	if d.APIKey != "" {
		if d.Provider == "gemini" {
			req.Header.Set("x-goog-api-key", d.APIKey)
		} else {
			req.Header.Set("Authorization", "Bearer "+d.APIKey)
		}
	}
	if d.Provider == "anthropic" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
	client := *s.Client
	client.Timeout = 30 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("could not reach the AI provider; check the server URL, connection, or local model server")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		switch response.StatusCode {
		case 401, 403:
			return nil, errors.New("the provider rejected this connection; check the API key and model access")
		case 429:
			return nil, errors.New("the AI provider rate or usage limit was reached; check your account and try later")
		case 404:
			return nil, errors.New("model or API endpoint not found; check the model ID and server URL")
		default:
			return nil, fmt.Errorf("AI provider returned HTTP %d; check the model and provider settings", response.StatusCode)
		}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 512*1024+1))
	if err != nil || len(data) > 512*1024 {
		return nil, errors.New("AI provider response was unreadable or too large")
	}
	return data, nil
}
func (s *Service) Complete(ctx context.Context, d Draft, instructions, input string) (string, error) {
	var body any
	path := ""
	messages := []map[string]string{{"role": "system", "content": instructions}, {"role": "user", "content": input}}
	switch d.Provider {
	case "openai":
		path = "/responses"
		body = map[string]any{"model": d.Model, "instructions": instructions, "input": input, "store": false, "max_output_tokens": 2048}
	case "anthropic":
		path = "/messages"
		body = map[string]any{"model": d.Model, "system": instructions, "messages": messages[1:], "max_tokens": 1024}
	case "gemini":
		path = "/models/" + url.PathEscape(d.Model) + ":generateContent"
		body = map[string]any{"systemInstruction": map[string]any{"parts": []map[string]string{{"text": instructions}}}, "contents": []any{map[string]any{"role": "user", "parts": []map[string]string{{"text": input}}}}, "generationConfig": map[string]any{"responseMimeType": "application/json", "maxOutputTokens": 2048}}
	case "ollama":
		path = "/api/chat"
		body = map[string]any{"model": d.Model, "messages": messages, "stream": false, "format": "json", "options": map[string]int{"num_predict": 1024}}
	case "compatible":
		path = "/chat/completions"
		body = map[string]any{"model": d.Model, "messages": messages, "max_tokens": 1024, "stream": false}
	default:
		return "", errors.New("unsupported AI provider")
	}
	data, err := s.request(ctx, d, http.MethodPost, path, body)
	if err != nil {
		return "", err
	}
	var response struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text    string `json:"text"`
					Thought bool   `json:"thought"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if json.Unmarshal(data, &response) != nil {
		return "", errors.New("the AI provider returned an unexpected response")
	}
	var text strings.Builder
	switch d.Provider {
	case "openai":
		for _, item := range response.Output {
			for _, part := range item.Content {
				if part.Type == "output_text" {
					text.WriteString(part.Text)
				}
			}
		}
	case "anthropic":
		for _, part := range response.Content {
			if part.Type == "text" {
				text.WriteString(part.Text)
			}
		}
	case "gemini":
		if len(response.Candidates) > 0 {
			for _, part := range response.Candidates[0].Content.Parts {
				if !part.Thought {
					text.WriteString(part.Text)
				}
			}
		}
	case "ollama":
		text.WriteString(response.Message.Content)
	case "compatible":
		if len(response.Choices) > 0 {
			text.WriteString(response.Choices[0].Message.Content)
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return "", errors.New("the model returned no text; choose a text-generation model and try again")
	}
	return text.String(), nil
}
func ParseJSON(text string, out any) error {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		if newline := strings.IndexByte(text, '\n'); newline >= 0 {
			text = text[newline+1:]
		}
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), "```"))
	}
	if json.Unmarshal([]byte(text), out) != nil {
		return errors.New("the model did not return valid JSON; try another text model")
	}
	return nil
}
func (s *Service) Test(ctx context.Context, d Draft) error {
	d, err := s.resolve(d, true)
	if err != nil {
		return err
	}
	text, err := s.Complete(ctx, d, `Return only the JSON object {"ok":true}.`, "Test the AI connection.")
	if err != nil {
		return err
	}
	var response struct {
		OK bool `json:"ok"`
	}
	if err = ParseJSON(text, &response); err != nil {
		return err
	}
	if !response.OK {
		return errors.New("the model did not pass the connection test; try another text model")
	}
	return nil
}
func (s *Service) Models(ctx context.Context, d Draft) ([]string, error) {
	d, err := s.resolve(d, false)
	if err != nil {
		return nil, err
	}
	path := "/models"
	if d.Provider == "ollama" {
		path = "/api/tags"
	}
	data, err := s.request(ctx, d, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			Name    string   `json:"name"`
			Methods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if json.Unmarshal(data, &response) != nil {
		return nil, errors.New("could not read the provider model list; enter a model ID manually")
	}
	models := []string{}
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" && !seen[name] && len(models) < 200 {
			seen[name] = true
			models = append(models, name)
		}
	}
	for _, model := range response.Data {
		if d.Provider == "openai" {
			name := strings.ToLower(model.ID)
			if !(strings.HasPrefix(name, "gpt-") || strings.HasPrefix(name, "o1") || strings.HasPrefix(name, "o3") || strings.HasPrefix(name, "o4") || strings.HasPrefix(name, "chatgpt-")) {
				continue
			}
			if strings.Contains(name, "image") || strings.Contains(name, "audio") || strings.Contains(name, "realtime") || strings.Contains(name, "transcri") {
				continue
			}
		}
		add(model.ID)
	}
	for _, model := range response.Models {
		if d.Provider == "gemini" {
			eligible := false
			for _, method := range model.Methods {
				if method == "generateContent" {
					eligible = true
				}
			}
			if !eligible {
				continue
			}
		}
		add(strings.TrimPrefix(model.Name, "models/"))
	}
	sort.Strings(models)
	return models, nil
}
