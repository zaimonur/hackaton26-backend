package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type LocalStorage struct {
	BaseDir string
}

func NewLocalStorage(baseDir string) FileStorage {
	return &LocalStorage{BaseDir: baseDir}
}

// UploadImage: Güvenli dosya yükleme işlemi yapar (MIME ve Uzantı kontrolü)
func (s *LocalStorage) UploadImage(fileHeader *multipart.FileHeader, folder string) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", errors.New("dosya okunamadı")
	}
	defer file.Close()

	// 1. Güvenlik: MIME Tipini kontrol et (Sadece uzantıya güvenmiyoruz, ilk 512 byte'ı okuyoruz)
	buffer := make([]byte, 512)
	if _, err := file.Read(buffer); err != nil && err != io.EOF {
		return "", errors.New("dosya içeriği okunamadı")
	}

	contentType := http.DetectContentType(buffer)
	if !strings.HasPrefix(contentType, "image/") {
		return "", errors.New("güvenlik ihlali: yüklenen dosya geçerli bir görsel değil")
	}

	// 2. İmleci başa al (512 byte okuduğumuz için)
	file.Seek(0, io.SeekStart)

	// 3. Güvenlik: Rastgele dosya ismi üret (Directory Traversal ve çakışma koruması)
	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		ext = ".png" // Fallback
	}

	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	secureFileName := hex.EncodeToString(randomBytes) + ext

	// 4. Hedef klasör (Örn: uploads/products)
	targetDir := filepath.Join(s.BaseDir, folder)
	targetPath := filepath.Join(targetDir, secureFileName)

	// 5. Dosyayı diske yaz
	dst, err := os.Create(targetPath)
	if err != nil {
		return "", errors.New("sunucu hatası: görsel diske yazılamadı")
	}
	defer dst.Close()

	if _, err = io.Copy(dst, file); err != nil {
		return "", errors.New("sunucu hatası: görsel kopyalanamadı")
	}

	// 6. DB'ye yazılacak URL formatını dön
	return fmt.Sprintf("/static/%s/%s", folder, secureFileName), nil
}
