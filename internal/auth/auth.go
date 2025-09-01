package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	// Hash the password using the bcrypt.GenerateFromPassword function
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

func VerifyPassword(hashedPassword, plainPassword string) (bool, error) {
	// Implementation for verifying the password
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
	if err != nil {
		return false, err
	}
	return true, nil
}

func MakeRefreshToken() (string, error) {
    rand_byte := make([]byte, 32) // 32 bytes for a secure token
    _, err := rand.Read(rand_byte)
    if err != nil {
        return "", err
    }
    rand_token_string := hex.EncodeToString(rand_byte)
    return rand_token_string, nil
}

func GetAPIKey(headers http.Header) (string, error) {
	// Expected header format Authorization: ApiKey THE_KEY_HERE
	apiKey := headers.Get("Authorization")
	if apiKey == "" {
		return "", errors.New("missing API key")
	}
	// Split the header value to extract the key
	parts := strings.SplitN(apiKey, " ", 2)
	if len(parts) != 2 || parts[0] != "ApiKey" {
		return "", errors.New("invalid API key format")
	}
	return parts[1], nil
}
