package auth

import (
	"crypto/rand"
	"encoding/hex"

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
