package ai

import (
	"bytes"
	"context"
	"drewisy/internal/domain"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
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

	// Gemini 2.5'in eklediği Markdown (```json) kalıntılarını temizle!
	rawString := string(text)
	rawString = strings.ReplaceAll(rawString, "```json", "")
	rawString = strings.ReplaceAll(rawString, "```", "")
	rawString = strings.TrimSpace(rawString)

	var matchedIDs []string
	if err := json.Unmarshal([]byte(rawString), &matchedIDs); err != nil {
		return nil, fmt.Errorf("ai yanıtı parse edilemedi: %v", err)
	}

	return matchedIDs, nil
}

func (s *geminiService) GenerateTextStream(ctx context.Context, prompt string) (<-chan string, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(s.apiKey))
	if err != nil {
		return nil, fmt.Errorf("gemini client oluşturulamadı: %v", err)
	}

	model := client.GenerativeModel("gemini-2.5-flash")
	iter := model.GenerateContentStream(ctx, genai.Text(prompt))

	chunkChan := make(chan string)

	go func() {
		defer client.Close()
		defer close(chunkChan) // Stream bittiğinde kanalı kapat

		for {
			select {
			case <-ctx.Done():
				return // Client bağlantıyı keserse veya timeout olursa çık
			default:
				resp, err := iter.Next()
				if err == iterator.Done {
					return
				}
				if err != nil {
					log.Printf("Gemini Stream Hatası: %v", err)
					return
				}

				if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
					part := resp.Candidates[0].Content.Parts[0]
					if text, ok := part.(genai.Text); ok {
						chunkChan <- string(text)
					}
				}
			}
		}
	}()

	return chunkChan, nil
}

func (s *geminiService) CreateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// text-embedding-004 modeline tam uyumlu endpoint
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent?key=%s", s.apiKey)

	reqBody := map[string]interface{}{
		"model": "models/gemini-embedding-001",
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
		return nil, fmt.Errorf("gemini embedding api hatası (HTTP %d)", resp.StatusCode)
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

func (s *geminiService) ParseSearchIntent(ctx context.Context, query string) (*domain.SearchIntent, error) {
	// Arama kısmı olduğu için müşteriyi bekletmemek adına 15 saniye timeout koyuyoruz
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctxWithTimeout, option.WithAPIKey(s.apiKey))
	if err != nil {
		return nil, err
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash")

	// Structured Output (JSON Schema)
	model.ResponseMIMEType = "application/json"
	model.ResponseSchema = &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"search_query": {
				Type:        genai.TypeString,
				Description: "Sadece aranan temel nesne veya ürün. Örn: 'kırmızı elbise', 'spor ayakkabı'",
			},
			"max_price": {
				Type:        genai.TypeNumber,
				Description: "Maksimum fiyat bütçesi. Metinde belirtilmemişse 0 dön.",
			},
			"in_stock_only": {
				Type:        genai.TypeBoolean,
				Description: "Metinde 'stokta', 'mevcut', 'hemen al' gibi stok vurgusu varsa true, aksi halde false dön.",
			},
		},
		Required: []string{"search_query", "max_price", "in_stock_only"},
	}

	prompt := fmt.Sprintf("Şu arama metninden kullanıcının niyetini çıkar: '%s'", query)
	resp, err := model.GenerateContent(ctxWithTimeout, genai.Text(prompt))
	if err != nil {
		return nil, err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, errors.New("ai yanıt veremedi")
	}

	part := resp.Candidates[0].Content.Parts[0]
	text, ok := part.(genai.Text)
	if !ok {
		return nil, errors.New("ai geçersiz format döndürdü")
	}

	rawString := string(text)
	rawString = strings.ReplaceAll(rawString, "```json", "")
	rawString = strings.ReplaceAll(rawString, "```", "")
	rawString = strings.TrimSpace(rawString)

	var intent domain.SearchIntent
	if err := json.Unmarshal([]byte(rawString), &intent); err != nil {
		return nil, err
	}

	return &intent, nil
}
