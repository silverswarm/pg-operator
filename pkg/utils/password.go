package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
)

const (
	passwordLength = 32
	specialChars   = "!@#$%^&*"
)

func GenerateSecurePassword() (string, error) {
	bytes := make([]byte, passwordLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func GenerateReadablePassword(length int) (string, error) {
	if length < 8 {
		length = 8
	}

	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	password := make([]byte, length-2)

	for i := range password {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random character: %w", err)
		}
		password[i] = charset[idx.Int64()]
	}

	// Add a digit and special character
	digitIdx, err := rand.Int(rand.Reader, big.NewInt(10))
	if err != nil {
		return "", err
	}

	specialIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(specialChars))))
	if err != nil {
		return "", err
	}

	result := string(password) + fmt.Sprintf("%d", digitIdx.Int64()) + string(specialChars[specialIdx.Int64()])
	return result, nil
}
