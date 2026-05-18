package ai

import (
	"bytes"
	"context"
	"drewisy/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type geminiService struct {
	apiKey string
}

// DI: API Key main.go üzerinden enjekte edilir
func NewGeminiService(apiKey string) domain.AIService {
	return &geminiService{
		apiKey: apiKey,
	}
}

func (s *geminiService) GenerateText(ctx context.Context, prompt string) (string, error) {
	// UI kilitlemesini önlemek için 10 saniyelik katı Timeout belirliyoruz
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Gemini Client oluşturma
	client, err := genai.NewClient(ctxWithTimeout, option.WithAPIKey(s.apiKey))
	if err != nil {
		return "", fmt.Errorf("gemini client oluşturulamadı: %v", err)
	}
	defer client.Close()

	// Hız ve maliyet verimliliği için gemini-2.5-flash modelini kullanıyoruz
	model := client.GenerativeModel("gemini-2.5-flash")

	// Gemini API'ye prompt'u gönderiyoruz
	resp, err := model.GenerateContent(ctxWithTimeout, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("ai yanıt üretirken hata oluştu: %v", err)
	}

	// Yanıtı ayrıştır ve sadece metin (string) olarak dön
	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		part := resp.Candidates[0].Content.Parts[0]
		if text, ok := part.(genai.Text); ok {
			return string(text), nil
		}
	}

	return "", errors.New("ai yanıtından geçerli bir metin alınamadı")
}

func (s *geminiService) SmartSearch(ctx context.Context, catalogJSON string, userQuery string) ([]string, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctxWithTimeout, option.WithAPIKey(s.apiKey))
	if err != nil {
		return nil, fmt.Errorf("gemini client oluşturulamadı: %v", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash")

	// Structured Output Zorlaması (Strict JSON Array)
	model.ResponseMIMEType = "application/json"
	model.ResponseSchema = &genai.Schema{
		Type: genai.TypeArray,
		Items: &genai.Schema{
			Type: genai.TypeString,
		},
	}

	prompt := fmt.Sprintf(`Kullanıcı e-ticaret uygulamasında şu aramayı yaptı: "%s".
Aşağıdaki ürün kataloğunu (JSON) incele. Sadece kullanıcının arama niyetiyle eşleşen ürünlerin ID'lerini döndür.
Katalog: %s`, userQuery, catalogJSON)

	resp, err := model.GenerateContent(ctxWithTimeout, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("ai arama başarısız: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return []string{}, nil
	}

	part := resp.Candidates[0].Content.Parts[0]
	text, ok := part.(genai.Text)
	if !ok {
		return nil, errors.New("ai geçersiz bir format döndürdü")
	}

	var matchedIDs []string
	if err := json.Unmarshal([]byte(text), &matchedIDs); err != nil {
		return nil, errors.New("ai yanıtı parse edilemedi")
	}

	return matchedIDs, nil
}

func (s *geminiService) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/text-embedding-004:embedContent?key=%s", s.apiKey)

	reqBody := map[string]interface{}{
		"model": "models/text-embedding-004",
		"content": map[string]interface{}{
			"parts": []map[string]string{
				{"text": text},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini embedding api hatası: %d", resp.StatusCode)
	}

	var result struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Embedding.Values, nil
}
