package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

var llmMu sync.Mutex

type LLMResult struct {
	Title   string
	Authors []string
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func RecognizeBook(firstPages, baseURL, model, token, prompt, prompt2 string, timeout int, debug bool) *LLMResult {
	result := llmCall(firstPages, baseURL, model, token, prompt, timeout, debug)
	if result != nil && result.Title == "" && len(result.Authors) == 0 {
		if prompt2 != "" {
			result = llmCall(firstPages, baseURL, model, token, prompt2, timeout, debug)
		}
	}
	return result
}

func llmCall(firstPages, baseURL, model, token, prompt string, timeout int, debug bool) *LLMResult {
	if timeout <= 0 {
		timeout = 60
	}
	llmMu.Lock()
	defer llmMu.Unlock()

	userContent := fmt.Sprintf("текст книги:\n%s", firstPages)

	reqBody := chatRequest{
		Model:     model,
		MaxTokens: 128,
		Messages: []chatMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: userContent},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil
	}

	apiURL := strings.TrimRight(baseURL, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	if debug {
		curl := fmt.Sprintf("curl -s -X POST '%s' -H 'Content-Type: application/json'", apiURL)
		if token != "" {
			curl += " -H 'Authorization: Bearer ***'"
		}
		curl += fmt.Sprintf(" -d '%s'", string(body))
		log.Printf("[LLM DEBUG] curl: %s", curl)
		log.Printf("[LLM DEBUG] request body: %s", string(body))
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		if debug {
			log.Printf("[LLM DEBUG] request failed: %v", err)
		}
		return nil
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		if debug {
			log.Printf("[LLM DEBUG] failed to read response: %v", err)
		}
		return nil
	}

	if debug {
		log.Printf("[LLM DEBUG] response status: %d", resp.StatusCode)
		log.Printf("[LLM DEBUG] response body: %s", string(respData))
	}

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respData, &chatResp); err != nil {
		if debug {
			log.Printf("[LLM DEBUG] failed to parse response JSON: %v", err)
		}
		return nil
	}

	if len(chatResp.Choices) == 0 {
		if debug {
			log.Printf("[LLM DEBUG] no choices in response")
		}
		return nil
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	if debug {
		log.Printf("[LLM DEBUG] parsed content: %s", content)
	}
	return parseLLMResponse(content)
}

func splitAuthors(text string) []string {
	parts := strings.Split(text, ",")
	var result []string
	for i := 0; i < len(parts); i++ {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		// Detect "Last, First" pattern: current part has no space (surname),
		// name following the comma is the given name (may be multi-word).
		if !strings.Contains(part, " ") && i+1 < len(parts) {
			next := strings.TrimSpace(parts[i+1])
			if next != "" {
				result = append(result, next+" "+part)
				i++
				continue
			}
		}
		result = append(result, part)
	}
	return result
}

func parseLLMResponse(content string) *LLMResult {
	result := &LLMResult{}

	lowerContent := strings.ToLower(content)
	authorIdx := strings.Index(lowerContent, "author:")
	bookIdx := strings.Index(lowerContent, "bookname:")

	if authorIdx >= 0 {
		start := authorIdx + len("AUTHOR:")
		end := len(content)
		if bookIdx > authorIdx {
			end = bookIdx
		}
		for _, a := range splitAuthors(content[start:end]) {
			if a != "" {
				result.Authors = append(result.Authors, a)
			}
		}
	} else if bookIdx > 0 {
		for _, a := range splitAuthors(content[:bookIdx]) {
			if a != "" {
				result.Authors = append(result.Authors, a)
			}
		}
	}

	if bookIdx >= 0 {
		start := bookIdx + len("BOOKNAME:")
		title := strings.TrimSpace(content[start:])
		if title != "" {
			result.Title = title
		}
	}

	if result.Title == "" && len(result.Authors) == 0 {
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			lowerLine := strings.ToLower(line)
			if strings.HasPrefix(lowerLine, "author:") {
				author := strings.TrimSpace(strings.TrimPrefix(line, "AUTHOR:"))
				if author != "" {
					for _, a := range splitAuthors(author) {
						if a != "" {
							result.Authors = append(result.Authors, a)
						}
					}
				}
			} else if strings.HasPrefix(lowerLine, "bookname:") {
				title := strings.TrimSpace(strings.TrimPrefix(line, "BOOKNAME:"))
				if title != "" {
					result.Title = title
				}
			}
		}
	}

	return result
}
