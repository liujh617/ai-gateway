package audit

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

var (
	ErrInvalidKeyLength = errors.New("encryption key must be 32 bytes")
	ErrInvalidCiphertext = errors.New("ciphertext is too short or invalid")
)

// Encryptor 定义审计日志加密器接口
type Encryptor interface {
	// Encrypt 加密明文，返回base64编码的密文
	Encrypt(plaintext []byte) (ciphertext string, err error)
	// Decrypt 解密base64编码的密文，返回明文
	Decrypt(ciphertext string) (plaintext []byte, err error)
}

// AES256GCMEncryptor AES-256-GCM加密器实现
type AES256GCMEncryptor struct {
	key []byte
}

// NewAES256GCMEncryptor 创建AES-256-GCM加密器
// key必须是32字节的字符串
func NewAES256GCMEncryptor(key string) (*AES256GCMEncryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("%w: got %d bytes", ErrInvalidKeyLength, len(key))
	}
	return &AES256GCMEncryptor{key: []byte(key)}, nil
}

// Encrypt 使用AES-256-GCM加密数据
// 返回base64编码的密文（包含nonce）
func (e *AES256GCMEncryptor) Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 加密数据（nonce + ciphertext）
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// 返回base64编码
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密AES-256-GCM加密的数据
// 输入base64编码的密文（包含nonce）
func (e *AES256GCMEncryptor) Decrypt(ciphertext string) ([]byte, error) {
	// base64解码
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	// 提取nonce和ciphertext
	nonce, encryptedData := data[:nonceSize], data[nonceSize:]

	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// NoopEncryptor 不加密的加密器（用于禁用加密的场景）
type NoopEncryptor struct{}

func NewNoopEncryptor() *NoopEncryptor {
	return &NoopEncryptor{}
}

func (e *NoopEncryptor) Encrypt(plaintext []byte) (string, error) {
	return string(plaintext), nil
}

func (e *NoopEncryptor) Decrypt(ciphertext string) ([]byte, error) {
	return []byte(ciphertext), nil
}