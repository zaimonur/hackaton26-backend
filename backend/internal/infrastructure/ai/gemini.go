package ai

import (
	"context"
	"drewisy/internal/domain"
	"errors"
	"fmt"
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
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
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
