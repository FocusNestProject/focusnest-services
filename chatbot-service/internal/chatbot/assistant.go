package chatbot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
)

// Assistant encapsulates model-backed responses.
type Assistant interface {
	Respond(ctx context.Context, lang string, prompt string, context []*ChatMessage) (string, error)
	Close() error
}

// AssistantConfig wires Gemini access.
type AssistantConfig struct {
	APIKey          string
	Model           string
	MaxOutputTokens int
	UseVertex       bool
	Project         string
	Location        string
}

// GeminiAssistant talks to Gemini 2.5 Flash.
type GeminiAssistant struct {
	client    *genai.Client
	model     string
	maxTokens int
}

// NewGeminiAssistant returns an Assistant backed by Gemini.
func NewGeminiAssistant(ctx context.Context, cfg AssistantConfig) (Assistant, error) {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "gemini-2.5-flash"
	}
	maxTokens := cfg.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	clientCfg := &genai.ClientConfig{}
	if cfg.UseVertex {
		project := strings.TrimSpace(cfg.Project)
		if project == "" {
			project = strings.TrimSpace(os.Getenv("GOOGLE_CLOUD_PROJECT"))
		}
		if project == "" {
			return nil, errors.New("vertex project id missing")
		}
		location := strings.TrimSpace(cfg.Location)
		if location == "" {
			location = strings.TrimSpace(os.Getenv("GOOGLE_CLOUD_LOCATION"))
		}
		if location == "" {
			return nil, errors.New("vertex location missing")
		}
		clientCfg.Project = project
		clientCfg.Location = location
		clientCfg.Backend = genai.BackendVertexAI
		if err := clientCfg.UseDefaultCredentials(); err != nil {
			return nil, fmt.Errorf("vertex credentials: %w", err)
		}
	} else {
		apiKey := strings.TrimSpace(cfg.APIKey)
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("GOOGLE_API_KEY"))
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
		}
		if apiKey == "" {
			return nil, errors.New("gemini api key missing")
		}
		clientCfg.APIKey = apiKey
		clientCfg.Backend = genai.BackendGeminiAPI
	}

	client, err := genai.NewClient(ctx, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("genai client: %w", err)
	}

	return &GeminiAssistant{client: client, model: model, maxTokens: maxTokens}, nil
}

// Close releases underlying Gemini resources.
func (g *GeminiAssistant) Close() error {
	return nil
}

// Respond generates a productivity-focused reply using prior context.
func (g *GeminiAssistant) Respond(ctx context.Context, lang string, prompt string, contextHistory []*ChatMessage) (string, error) {
	contents := make([]*genai.Content, 0, len(contextHistory)+1)
	for _, msg := range contextHistory {
		contents = append(contents, genai.NewContentFromText(msg.Content, roleForMessage(msg.Role)))
	}
	contents = append(contents, genai.NewContentFromText(prompt, genai.RoleUser))

	resp, err := g.client.Models.GenerateContent(ctx, g.model, contents, &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemPrompt(lang), genai.RoleUser),
		Temperature:       genai.Ptr(float32(0.75)),
		TopP:              genai.Ptr(float32(0.95)),
		MaxOutputTokens:   int32(g.maxTokens),
	})
	if err != nil {
		return "", err
	}
	output := strings.TrimSpace(resp.Text())
	if output == "" {
		return "", errors.New("gemini returned empty response")
	}
	return output, nil
}

func roleForMessage(role string) genai.Role {
	if role == "assistant" {
		return genai.RoleModel
	}
	return genai.RoleUser
}

// TemplateAssistant is a fallback when Gemini is unavailable.
type TemplateAssistant struct{}

// NewTemplateAssistant returns a deterministic responder using heuristics.
func NewTemplateAssistant() Assistant {
	return &TemplateAssistant{}
}

// Respond falls back to the built-in productivity template.
func (t *TemplateAssistant) Respond(ctx context.Context, lang string, prompt string, contextHistory []*ChatMessage) (string, error) {
	return buildProductivityResponse(prompt, contextHistory, lang), nil
}

// Close is a no-op for the template assistant.
func (t *TemplateAssistant) Close() error { return nil }

func systemPrompt(lang string) string {
	base := `You are FocusNest, a warm and conversational productivity coach. Your role is to help users with focus, deep work, habits, routines, study techniques, healthy rest, and motivation.

Key principles:
- Be natural and conversational, not robotic or template-like
- Pay close attention to the conversation history and reference specific things the user mentioned earlier
- Adapt your tone and approach based on the user's mood and context from previous messages
- If the user seems stressed, be empathetic. If they're excited, match their energy. If they're confused, be clear and patient
- Reference specific details from earlier in the conversation to show you're listening
- Keep responses concise but warm—aim for 2-4 sentences or a few bullet points, not rigid templates
- If the topic drifts, gently acknowledge it and offer a brief productivity connection, but don't be dismissive
- Use the conversation flow naturally—build on what was said before, ask follow-up questions when appropriate, and maintain continuity

Remember: You're having a conversation, not delivering a script. Let the context guide your response style and content.`
	if lang == languageIndonesian {
		base += ` Jawab dalam Bahasa Indonesia yang santai dan natural, seperti ngobrol dengan teman yang peduli. Gunakan konteks percakapan sebelumnya untuk membuat respons yang relevan dan personal.`
	} else {
		base += ` Reply in natural, conversational English that feels personal and context-aware.`
	}
	return base
}
