package storage

import "mime/multipart"

// FileStorage: Dosya yükleme işlemlerini soyutlayan arayüz.
// S3'e geçmek istersen sadece bu interface'i uygulayan yeni bir struct yazacaksın.
type FileStorage interface {
	UploadImage(fileHeader *multipart.FileHeader, folder string) (string, error)
}
