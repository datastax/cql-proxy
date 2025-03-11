package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"crypto/aes"
	"crypto/cipher"
)

func decrypt(encrypted, key string) (string, error) {
	// Decode the Base64 key
	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("base64 decode error (key): %v", err)
	}

	// Ensure the key length is valid for AES
	if len(keyBytes) != 16 && len(keyBytes) != 24 && len(keyBytes) != 32 {
		return "", fmt.Errorf("invalid AES key size: %d bytes", len(keyBytes))
	}

	// Decode the Base64 encrypted data
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("base64 decode error (ciphertext): %v", err)
	}

	// Ensure ciphertext contains at least the IV (16 bytes)
	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short (length: %d, expected >= %d)", len(ciphertext), aes.BlockSize)
	}

	// Extract IV and ciphertext
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	// Create AES cipher
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", fmt.Errorf("new cipher error: %v", err)
	}

	// Decrypt using CFB mode
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run decrypt.go <encrypted-text>")
		return
	}

	encryptedText := os.Args[1]
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		fmt.Println("ENCRYPTION_KEY environment variable is not set.")
		return
	}

	decryptedText, err := decrypt(encryptedText, encryptionKey)
	if err != nil {
		fmt.Println("Error decrypting:", err)
		return
	}

	fmt.Println("Decrypted text:", decryptedText)
}
