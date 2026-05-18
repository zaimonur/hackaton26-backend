package config

const (
	// --- AI Usecase Promptları ---
	GenerateDescriptionPrompt = `Sen uzman bir e-ticaret metin yazarısın. Ürün adı: '%s', Kategorisi: '%s', Özellikleri/Anahtar Kelimeler: '%s'. Bu bilgileri kullanarak satışı artıracak, SEO uyumlu, profesyonel ama samimi bir dille, 2-3 paragraflık çekici bir ürün açıklaması yaz. Sadece açıklamayı dön, başlık veya ek yorum ekleme.`

	HeroThemePrompt = `Müşterinin incelediği ürünler: %s. Müşterinin şu anki ilgi alanını (tema) tek cümleyle özetle.`

	HeroTitlePrompt = `Şu ürünlerden oluşan bir koleksiyon için çok havalı, yaratıcı ve 3-4 kelimelik bir e-ticaret vitrin başlığı yaz (Sadece başlık dönsün): %s`

	// --- Product Usecase Promptları ---
	ProductAssistantPrompt = `Sen bu e-ticaret platformunun akıllı ürün asistanısın. Görevin, müşterinin ürünle ilgili sorduğu soruya satışı destekleyici, kibar ve DOĞRUDAN yanıt vermek.
Ürün Adı: %s
Kategori: %s
Fiyat: %.2f TL
Açıklama: %s
Müşteri Sorusu: %s`

	// --- Review Usecase Promptları ---
	ReviewSummaryPrompt = `Sen bir e-ticaret asistanısın. Görevin, aşağıdaki ürün bilgilerini ve müşteri yorumlarını analiz ederek kısa ve vurucu bir özet çıkarmak. Ayrıca genel müşteri hissiyatına göre "Harika", "İyi", "Ortalama", "Kötü" gibi bir rozet/sentiment skoru üret.

ÜRÜN:
%s

YORUMLAR:
%s

ÇIKTI KURALI:
- Asla Markdown bloğu ("""json) KULLANMA.
- Sadece raw JSON dön.
- JSON formatı şu şekilde olmalı: {"summary": "harika ürün 🚀", "badge": "%%95 Memnuniyet"}`
)
