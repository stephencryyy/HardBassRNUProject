package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

type ChecksumService struct{}

// NewChecksumService создает новый экземпляр ChecksumService.
func NewChecksumService() *ChecksumService {
	return &ChecksumService{}
}

// ValidateChecksum проверяет, совпадает ли контрольная сумма данных с ожидаемой.
func (s *ChecksumService) ValidateChecksum(data []byte, expectedChecksum string) bool {
	actualChecksum := s.CalculateChecksum(data)
	return actualChecksum == expectedChecksum
}

// CalculateChecksum вычисляет контрольную сумму данных с использованием SHA-256.
func (s *ChecksumService) CalculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
