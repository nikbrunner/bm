package ai

// Response represents the AI-suggested bookmark metadata.
type Response struct {
	Title      string   `json:"title"`
	FolderPath string   `json:"folderPath"`
	Tags       []string `json:"tags"`
}

// OrganizeResponse represents the AI-suggested organization for an item.
type OrganizeResponse struct {
	FolderPath    string   `json:"folderPath"`
	IsNewFolder   bool     `json:"isNewFolder"`
	SuggestedTags []string `json:"suggestedTags"` // suggested tags (bookmarks only)
	Confidence    string   `json:"confidence"`    // "high", "medium", "low"
}

// apiRequest represents the Anthropic API request body.
type apiRequest struct {
	Model        string        `json:"model"`
	MaxTokens    int           `json:"max_tokens"`
	Messages     []apiMessage  `json:"messages"`
	OutputFormat *outputFormat `json:"output_format,omitempty"`
}

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type outputFormat struct {
	Type   string     `json:"type"`
	Schema jsonSchema `json:"schema"`
}

type jsonSchema struct {
	Type                 string                `json:"type"`
	Properties           map[string]schemaProp `json:"properties"`
	Required             []string              `json:"required"`
	AdditionalProperties bool                  `json:"additionalProperties"`
}

type schemaProp struct {
	Type  string      `json:"type"`
	Items *schemaProp `json:"items,omitempty"`
}

// apiResponse represents the Anthropic API response body.
type apiResponse struct {
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
