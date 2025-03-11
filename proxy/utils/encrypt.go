package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

// Encrypt encrypts plaintext using AES-CFB and returns a Base64-encoded ciphertext.
func encrypt(plaintext, key string) (string, error) {
	// Decode the Base64 key
	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("base64 decode error (key): %v", err)
	}

	// Validate AES key size
	if len(keyBytes) != 16 && len(keyBytes) != 24 && len(keyBytes) != 32 {
		return "", fmt.Errorf("invalid AES key size: %d bytes", len(keyBytes))
	}

	// Generate a random IV
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("IV generation error: %v", err)
	}

	// Create AES cipher block
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("AES cipher creation error: %v", err)
	}

	// Encrypt using CFB mode
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(plaintext))

	// Return Base64-encoded ciphertext
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run encrypt.go <plaintext>")
		return
	}

	plaintext := os.Args[1]
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		fmt.Println("Error: ENCRYPTION_KEY is not set")
		return
	}

	encrypted, err := encrypt(plaintext, encryptionKey)
	if err != nil {
		fmt.Printf("Encryption failed: %v\n", err)
		return
	}

	fmt.Printf("Encrypted: %s\n", encrypted)
}
