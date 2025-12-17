package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	apiURL     = "https://api.anthropic.com/v1/messages"
	apiVersion = "2023-06-01"
	betaHeader = "structured-outputs-2025-11-13"
	haikuModel = "claude-haiku-4-5-20251001"
)

var (
	ErrNoAPIKey        = errors.New("ANTHROPIC_API_KEY environment variable not set")
	ErrAPIRequest      = errors.New("API request failed")
	ErrInvalidResponse = errors.New("invalid API response")
)

// Client handles communication with the Anthropic API.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new AI client.
// Returns an error if ANTHROPIC_API_KEY is not set.
func NewClient() (*Client, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, ErrNoAPIKey
	}

	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// SuggestBookmark calls the AI to suggest title, folder, and tags for a URL.
func (c *Client) SuggestBookmark(url string, context string) (*Response, error) {
	prompt := buildPrompt(url, context)

	reqBody := apiRequest{
		Model:     haikuModel,
		MaxTokens: 256,
		Messages: []apiMessage{
			{Role: "user", Content: prompt},
		},
		OutputFormat: &outputFormat{
			Type: "json_schema",
			Schema: jsonSchema{
				Type: "object",
				Properties: map[string]schemaProp{
					"title":      {Type: "string"},
					"folderPath": {Type: "string"},
					"tags":       {Type: "array", Items: &schemaProp{Type: "string"}},
				},
				Required:             []string{"title", "folderPath", "tags"},
				AdditionalProperties: false,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", apiVersion)
	req.Header.Set("anthropic-beta", betaHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAPIRequest, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d: %s", ErrAPIRequest, resp.StatusCode, string(body))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(apiResp.Content) == 0 || apiResp.Content[0].Type != "text" {
		return nil, ErrInvalidResponse
	}

	var result Response
	if err := json.Unmarshal([]byte(apiResp.Content[0].Text), &result); err != nil {
		return nil, fmt.Errorf("unmarshal AI response: %w", err)
	}

	return &result, nil
}

// SuggestOrganize calls the AI to suggest folder and tags for an item.
func (c *Client) SuggestOrganize(title, url, currentPath string, tags []string, isFolder bool, context string) (*OrganizeResponse, error) {
	prompt := buildOrganizePrompt(title, url, currentPath, tags, isFolder, context)

	reqBody := apiRequest{
		Model:     haikuModel,
		MaxTokens: 256,
		Messages: []apiMessage{
			{Role: "user", Content: prompt},
		},
		OutputFormat: &outputFormat{
			Type: "json_schema",
			Schema: jsonSchema{
				Type: "object",
				Properties: map[string]schemaProp{
					"folderPath":    {Type: "string"},
					"isNewFolder":   {Type: "boolean"},
					"suggestedTags": {Type: "array", Items: &schemaProp{Type: "string"}},
					"confidence":    {Type: "string"},
				},
				Required:             []string{"folderPath", "isNewFolder", "suggestedTags", "confidence"},
				AdditionalProperties: false,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", apiVersion)
	req.Header.Set("anthropic-beta", betaHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAPIRequest, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d: %s", ErrAPIRequest, resp.StatusCode, string(body))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(apiResp.Content) == 0 || apiResp.Content[0].Type != "text" {
		return nil, ErrInvalidResponse
	}

	var result OrganizeResponse
	if err := json.Unmarshal([]byte(apiResp.Content[0].Text), &result); err != nil {
		return nil, fmt.Errorf("unmarshal AI response: %w", err)
	}

	return &result, nil
}

func buildPrompt(url string, context string) string {
	return fmt.Sprintf(`Analyze this URL and suggest a title, folder, and tags for bookmarking.

URL: %s

%s

Instructions:
- Suggest a concise, descriptive title for this bookmark
- Choose the most appropriate folder path from the available folders
- If no folder fits well, use "/To Review"
- Suggest 1-3 relevant tags, preferring existing tags when they fit
- If suggesting new tags, keep them lowercase and concise`, url, context)
}

func buildOrganizePrompt(title, url, currentPath string, tags []string, isFolder bool, context string) string {
	itemType := "bookmark"
	if isFolder {
		itemType = "folder"
	}

	tagsStr := ""
	if len(tags) > 0 {
		tagsStr = fmt.Sprintf("\n- Current tags: %s", strings.Join(tags, ", "))
	}

	urlStr := ""
	if url != "" {
		urlStr = fmt.Sprintf("\n- URL: %s", url)
	}

	tagInstructions := ""
	if !isFolder {
		tagInstructions = `
- Suggest 1-3 relevant tags for this bookmark
- Prefer existing tags when they fit well
- If current tags are already optimal, return them as-is
- Keep tags lowercase and concise
- Return empty array only if no tags are appropriate`
	} else {
		tagInstructions = "\n- Return empty array for suggestedTags (folders don't have tags)"
	}

	return fmt.Sprintf(`Analyze this %s and suggest the best organization.

Item:
- Title: %s%s
- Current folder: %s%s

%s

Instructions:
- Prefer existing folders when they fit well
- Only suggest a new folder path if nothing existing is appropriate
- Set isNewFolder=true only when suggesting a folder that doesn't exist
- If current location is already optimal, return the current path exactly
- Confidence: "high" if clear match, "medium" if reasonable, "low" if uncertain%s`,
		itemType, title, urlStr, currentPath, tagsStr, context, tagInstructions)
}
