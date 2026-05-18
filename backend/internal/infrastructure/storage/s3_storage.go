package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

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

// UploadImage: Klasik (Yavaş ve RAM tüketen) yükleme metodu
func (s *S3Storage) UploadImage(fileHeader *multipart.FileHeader, folder string) (string, error) {
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
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return "", errors.New("dosya belleğe alınamadı")
	}

	ext := filepath.Ext(fileHeader.Filename)
	if ext == "" {
		ext = ".png"
	}

	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	secureFileName := hex.EncodeToString(randomBytes) + ext

	s3Key := fmt.Sprintf("%s/%s", folder, secureFileName)

	svc := s3.New(s.Session)
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(s.Bucket),
		Key:         aws.String(s3Key),
		Body:        bytes.NewReader(fileBytes),
		ContentType: aws.String(contentType),
		ACL:         aws.String("public-read"),
	})

	if err != nil {
		return "", fmt.Errorf("s3'e yükleme başarısız: %v", err)
	}

	fileURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.Bucket, s.Region, s3Key)
	return fileURL, nil
}

// YENİ EKLENEN METOT: GeneratePresignedURL (Bant Genişliği Koruyucu)
func (s *S3Storage) GeneratePresignedURL(ctx context.Context, folder string, filename string) (string, string, error) {
	// Çakışmayı önlemek için orijinal dosya adının sadece uzantısını alıp rastgele isim üretiyoruz
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".png"
	}

	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	secureFileName := hex.EncodeToString(randomBytes) + ext

	// S3 içindeki tam yol (Örn: products/f7a1...8b.png)
	s3Key := fmt.Sprintf("%s/%s", folder, secureFileName)

	svc := s3.New(s.Session)
	req, _ := svc.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(s3Key),
		// ACL (Access Control List) bucket ayarlarında block'lanmış olabilir, hata alırsan bu satırı yoruma alabilirsin
		ACL: aws.String("public-read"),
	})

	// 15 dakika boyunca geçerli olacak PUT URL'sini üret
	uploadURL, err := req.Presign(15 * time.Minute)
	if err != nil {
		return "", "", fmt.Errorf("presigned url üretilemedi: %v", err)
	}

	// Yükleme bittikten sonra DB'ye kaydedilecek olan kalıcı public URL
	finalURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.Bucket, s.Region, s3Key)

	return uploadURL, finalURL, nil
}
