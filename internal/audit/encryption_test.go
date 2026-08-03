package audit

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func TestAES256GCMEncryptor_EncryptDecrypt(t *testing.T) {
	// 生成32字节密钥
	key := generateTestKey(t)

	encryptor, err := NewAES256GCMEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{"empty", ""},
		{"single_char", "a"},
		{"short", "hello"},
		{"medium", "This is a test message with some content"},
		{"long", strings.Repeat("This is a long test message. ", 100)},
		{"json", `{"messages":[{"role":"user","content":"Hello"}],"model":"gpt-4"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plaintext := []byte(tt.plaintext)

			// 加密
			ciphertext, err := encryptor.Encrypt(plaintext)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			// 验证密文不为空（且不同于明文）
			if tt.plaintext != "" && ciphertext == tt.plaintext {
				t.Error("ciphertext should differ from plaintext")
			}

			// 解密
			decrypted, err := encryptor.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			// 验证解密结果
			if string(decrypted) != tt.plaintext {
				t.Errorf("Decrypt() = %q, want %q", decrypted, plaintext)
			}
		})
	}
}

func TestAES256GCMEncryptor_InvalidKeyLength(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"too_short", "short"},
		{"too_long", strings.Repeat("x", 64)},
		{"31_bytes", strings.Repeat("x", 31)},
		{"33_bytes", strings.Repeat("x", 33)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAES256GCMEncryptor(tt.key)
			if err == nil {
				t.Error("expected error for invalid key length")
			}
			if err != ErrInvalidKeyLength {
				t.Errorf("expected ErrInvalidKeyLength, got %v", err)
			}
		})
	}
}

func TestAES256GCMEncryptor_WrongKey(t *testing.T) {
	key1 := generateTestKey(t)
	key2 := generateTestKey(t)

	encryptor1, err := NewAES256GCMEncryptor(key1)
	if err != nil {
		t.Fatalf("failed to create encryptor1: %v", err)
	}

	encryptor2, err := NewAES256GCMEncryptor(key2)
	if err != nil {
		t.Fatalf("failed to create encryptor2: %v", err)
	}

	plaintext := []byte("secret message")

	// 用key1加密
	ciphertext, err := encryptor1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// 用key2解密（应该失败）
	_, err = encryptor2.Decrypt(ciphertext)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

func TestAES256GCMEncryptor_InvalidCiphertext(t *testing.T) {
	key := generateTestKey(t)

	encryptor, err := NewAES256GCMEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	tests := []struct {
		name      string
		ciphertext string
	}{
		{"empty", ""},
		{"invalid_base64", "not-base64-!!!"},
		{"too_short", base64.StdEncoding.EncodeToString([]byte("short"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := encryptor.Decrypt(tt.ciphertext)
			if err == nil {
				t.Error("expected error for invalid ciphertext")
			}
		})
	}
}

func TestAES256GCMEncryptor_UniqueCiphertext(t *testing.T) {
	key := generateTestKey(t)

	encryptor, err := NewAES256GCMEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	plaintext := []byte("same message")

	// 加密两次
	ciphertext1, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("first Encrypt() error = %v", err)
	}

	ciphertext2, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("second Encrypt() error = %v", err)
	}

	// 密文应该不同（因为随机nonce）
	if ciphertext1 == ciphertext2 {
		t.Error("same plaintext should produce different ciphertexts")
	}

	// 但解密后应该相同
	decrypted1, err := encryptor.Decrypt(ciphertext1)
	if err != nil {
		t.Fatalf("first Decrypt() error = %v", err)
	}

	decrypted2, err := encryptor.Decrypt(ciphertext2)
	if err != nil {
		t.Fatalf("second Decrypt() error = %v", err)
	}

	if string(decrypted1) != string(decrypted2) {
		t.Error("decrypted messages should be equal")
	}
}

func TestAES256GCMEncryptor_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	key := generateTestKey(t)

	encryptor, err := NewAES256GCMEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	// 生成10KB数据（典型审计日志大小）
	plaintext := make([]byte, 10*1024)
	rand.Read(plaintext)

	// 测试加密性能
	iterations := 1000
	for i := 0; i < iterations; i++ {
		ciphertext, err := encryptor.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt() error = %v", err)
		}

		// 解密验证
		decrypted, err := encryptor.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("Decrypt() error = %v", err)
		}

		if len(decrypted) != len(plaintext) {
			t.Errorf("decrypted length = %d, want %d", len(decrypted), len(plaintext))
		}
	}

	// 如果到这里没有错误，性能测试通过
	t.Logf("Successfully encrypted/decrypted %d iterations of 10KB data", iterations)
}

func TestNoopEncryptor(t *testing.T) {
	encryptor := NewNoopEncryptor()

	plaintext := []byte("test message")

	ciphertext, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if ciphertext != string(plaintext) {
		t.Errorf("Encrypt() = %q, want %q", ciphertext, plaintext)
	}

	decrypted, err := encryptor.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

// 辅助函数：生成测试密钥
func generateTestKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	return string(key)
}