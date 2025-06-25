package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func GenerarSalt() ([]byte, error) {
	salt := make([]byte, 16)
	_, err := rand.Read(salt)
	return salt, err
}

func HashConSalt(password string, salt []byte) string {
	hash := sha256.Sum256(append([]byte(password), salt...))
	return hex.EncodeToString(hash[:])
}
