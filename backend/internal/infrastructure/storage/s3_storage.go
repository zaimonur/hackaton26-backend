package storage

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3Storage struct {
	Session *session.Session
	Bucket  string
	Region  string
}

// NewS3Storage: Yeni bir S3 bağlantısı oluşturur
func NewS3Storage(region string, bucket string) (FileStorage, error) {
	// AWS credential'larını otomatik olarak environment variable'lardan alır
	// (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})

	if err != nil {
		return nil, fmt.Errorf("s3 oturumu oluşturulamadı: %v", err)
	}

	return &S3Storage{
		Session: sess,
		Bucket:  bucket,
		Region:  region,
	}, nil
}

// UploadImage: Dosyayı AWS S3'e (veya MinIO'ya) yükler
func (s *S3Storage) UploadImage(fileHeader *multipart.FileHeader, folder string) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", errors.New("dosya okunamadı")
	}
	defer file.Close()

	// 1. MIME Tipi Kontrolü (Güvenlik)
	buffer := make([]byte, 512)
	if _, err := file.Read(buffer); err != nil && err != io.EOF {
		return "", errors.New("dosya içeriği okunamadı")
	}

	contentType := http.DetectContentType(buffer)
	if !strings.HasPrefix(contentType, "image/") {
		return "", errors.New("güvenlik ihlali: yüklenen dosya geçerli bir görsel değil")
	}

	// İmleci başa al ve dosyanın tamamını belleğe oku (S3'e göndermek için gerekli)
	file.Seek(0, io.SeekStart)
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return "", errors.New("dosya belleğe alınamadı")
	}

	// 2. Rastgele Dosya İsmi (Çakışma koruması)
	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		ext = ".png"
	}

	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	secureFileName := hex.EncodeToString(randomBytes) + ext

	// S3'teki klasör yolu (Örn: products/a1b2c3d4.png)
	s3Key := fmt.Sprintf("%s/%s", folder, secureFileName)

	// 3. S3'e Yükleme İşlemi
	svc := s3.New(s.Session)
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(s.Bucket),
		Key:         aws.String(s3Key),
		Body:        bytes.NewReader(fileBytes),
		ContentType: aws.String(contentType),
		ACL:         aws.String("public-read"), // Dosyanın URL üzerinden okunabilmesi için
	})

	if err != nil {
		return "", fmt.Errorf("s3'e yükleme başarısız: %v", err)
	}

	// 4. S3 URL'sini dön
	// AWS formatı: https://{bucket-name}.s3.{region}.amazonaws.com/{key}
	fileURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.Bucket, s.Region, s3Key)
	return fileURL, nil
}
