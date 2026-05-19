package storage

import (
	"context"
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

	buffer := make([]byte, 512)
	if _, err := file.Read(buffer); err != nil && err != io.EOF {
		return "", errors.New("dosya içeriği okunamadı")
	}

	contentType := http.DetectContentType(buffer)
	if !strings.HasPrefix(contentType, "image/") {
		return "", errors.New("güvenlik ihlali: yüklenen dosya geçerli bir görsel değil")
	}

	file.Seek(0, io.SeekStart)

	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		ext = ".png"
	}

	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	secureFileName := hex.EncodeToString(randomBytes) + ext

	targetDir := filepath.Join(s.BaseDir, folder)
	targetPath := filepath.Join(targetDir, secureFileName)

	dst, err := os.Create(targetPath)
	if err != nil {
		return "", errors.New("sunucu hatası: görsel diske yazılamadı")
	}
	defer dst.Close()

	if _, err = io.Copy(dst, file); err != nil {
		return "", errors.New("sunucu hatası: görsel kopyalanamadı")
	}

	return fmt.Sprintf("/static/%s/%s", folder, secureFileName), nil
}

// Interface zorunluluğu
func (s *LocalStorage) GeneratePresignedURL(ctx context.Context, folder string, filename string) (string, string, error) {
	// Pre-signed URL mimarisi sadece S3, MinIO, GCS gibi bulut tabanlı depolamalarda çalışır.
	// Geliştirme ortamında bu fonksiyona düşülürse developer'ı uyarıyoruz.
	return "", "", errors.New("Lokal depolama (local_storage) Pre-signed URL mimarisini desteklemez. Lütfen .env dosyasında USE_S3=true yapın")
}
