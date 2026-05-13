package usecase

import (
	"context"
	"drewisy/internal/domain"
	"errors"
	"fmt"
	"strings"
)

type aiUsecase struct {
	aiService domain.AIService
}

// DI: AIService içeri enjekte ediliyor
func NewAIUsecase(aiService domain.AIService) domain.AIUsecase {
	return &aiUsecase{
		aiService: aiService,
	}
}

func (u *aiUsecase) GenerateDescription(ctx context.Context, req *domain.GenerateDescriptionRequest) (*domain.GenerateDescriptionResponse, error) {
	req.Title = strings.TrimSpace(req.Title)
	req.Category = strings.TrimSpace(req.Category)
	req.Keywords = strings.TrimSpace(req.Keywords)

	if req.Title == "" || req.Category == "" {
		return nil, errors.New("ürün adı ve kategorisi zorunludur")
	}

	// [KRİTİK - PROMPT ENGINEERING]
	prompt := fmt.Sprintf(
		"Sen uzman bir e-ticaret metin yazarısın. Ürün adı: '%s', Kategorisi: '%s', Özellikleri/Anahtar Kelimeler: '%s'. "+
			"Bu bilgileri kullanarak satışı artıracak, SEO uyumlu, profesyonel ama samimi bir dille, "+
			"en fazla 2-3 kısa paragraf olacak şekilde bir ürün açıklaması yaz. "+
			"Çıktı sadece açıklama metni olsun, gereksiz sohbet, selamlama veya markdown başlıkları ekleme.",
		req.Title, req.Category, req.Keywords,
	)

	// AIService (Infrastructure) çağrısı
	generatedText, err := u.aiService.GenerateText(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Sadece JSON (DTO) dönecek, veritabanına kayıt yok
	return &domain.GenerateDescriptionResponse{
		GeneratedDescription: strings.TrimSpace(generatedText),
	}, nil
}
