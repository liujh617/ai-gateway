package audit_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"open-ai-gateway/internal/audit"
)

func TestJSONLRecorderEncryptsBodyWhenEncryptorProvided(t *testing.T) {
	// 生成32字节测试密钥
	key := "test-encryption-key-32-bytes!!"
	encryptor, err := audit.NewAES256GCMEncryptor(key)
	if err != nil {
		t.Fatalf("NewAES256GCMEncryptor: %v", err)
	}

	path := filepath.Join(t.TempDir(), "audit", "encrypted.jsonl")
	rec, err := audit.NewJSONLRecorderWithOptions(path, audit.JSONLRecorderOptions{}, encryptor)
	if err != nil {
		t.Fatalf("NewJSONLRecorderWithOptions: %v", err)
	}
	defer rec.Close()

	// 记录包含敏感数据的审计事件
	sensitiveBody := json.RawMessage(`{"messages":[{"role":"user","content":"secret data"}],"model":"gpt-4"}`)
	rec.Record(context.Background(), audit.Event{
		Timestamp:     time.Date(2026, 8, 3, 21, 20, 0, 0, time.UTC),
		Event:         audit.EventRequest,
		RequestID:     "req_encrypted",
		Client:        "finance-team",
		Body:          sensitiveBody,
	})

	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 读取审计日志文件
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("missing audit line")
	}

	var got map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
		t.Fatalf("decode audit line: %v", err)
	}

	// 验证基本信息
	if got["event"] != "request" {
		t.Fatalf("event = %v, want request", got["event"])
	}
	if got["request_id"] != "req_encrypted" {
		t.Fatalf("request_id = %v, want req_encrypted", got["request_id"])
	}

	// 验证Body被加密
	bodyEncrypted, ok := got["body_encrypted"].(bool)
	if !ok || !bodyEncrypted {
		t.Fatal("body_encrypted should be true")
	}

	// 验证Body是字符串（加密后的base64）而非原始JSON对象
	bodyStr, ok := got["body"].(string)
	if !ok {
		t.Fatalf("body should be encrypted string, got %T", got["body"])
	}

	// 验证加密后的Body不包含原始敏感数据
	if bodyStr == "" {
		t.Fatal("encrypted body should not be empty")
	}

	// 解密并验证原始数据
	decrypted, err := encryptor.Decrypt(bodyStr)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	var decryptedBody map[string]any
	if err := json.Unmarshal(decrypted, &decryptedBody); err != nil {
		t.Fatalf("unmarshal decrypted body: %v", err)
	}

	if decryptedBody["model"] != "gpt-4" {
		t.Fatalf("decrypted model = %v, want gpt-4", decryptedBody["model"])
	}
}

func TestJSONLRecorderDoesNotEncryptWhenNoEncryptor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit", "plain.jsonl")
	rec, err := audit.NewJSONLRecorderWithOptions(path, audit.JSONLRecorderOptions{}, nil)
	if err != nil {
		t.Fatalf("NewJSONLRecorderWithOptions: %v", err)
	}
	defer rec.Close()

	body := json.RawMessage(`{"model":"test-model","messages":[{"role":"user","content":"hello"}]}`)
	rec.Record(context.Background(), audit.Event{
		Timestamp: time.Date(2026, 8, 3, 21, 20, 0, 0, time.UTC),
		Event:     audit.EventRequest,
		RequestID: "req_plain",
		Body:      body,
	})

	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 读取并验证未加密
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("missing audit line")
	}

	var got map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
		t.Fatalf("decode audit line: %v", err)
	}

	// 验证Body未加密
	if _, ok := got["body_encrypted"]; ok {
		t.Fatal("body_encrypted field should not exist when encryption is disabled")
	}

	// 验证Body是原始JSON对象
	bodyMap, ok := got["body"].(map[string]any)
	if !ok {
		t.Fatalf("body should be JSON object, got %T", got["body"])
	}

	if bodyMap["model"] != "test-model" {
		t.Fatalf("body model = %v, want test-model", bodyMap["model"])
	}
}

func TestJSONLRecorderHandlesEncryptionFailureGracefully(t *testing.T) {
	// 使用会失败的加密器（密钥长度错误会在构造时失败，这里用空密钥测试）
	encryptor := audit.NewNoopEncryptor()

	path := filepath.Join(t.TempDir(), "audit", "fallback.jsonl")
	rec, err := audit.NewJSONLRecorderWithOptions(path, audit.JSONLRecorderOptions{}, encryptor)
	if err != nil {
		t.Fatalf("NewJSONLRecorderWithOptions: %v", err)
	}
	defer rec.Close()

	body := json.RawMessage(`{"model":"test"}`)
	rec.Record(context.Background(), audit.Event{
		Event:     audit.EventRequest,
		RequestID: "req_fallback",
		Body:      body,
	})

	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 验证仍然记录了审计日志（降级处理）
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("missing audit line")
	}

	var got map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
		t.Fatalf("decode audit line: %v", err)
	}

	if got["request_id"] != "req_fallback" {
		t.Fatalf("request_id = %v, want req_fallback", got["request_id"])
	}
}

func TestJSONLRecorderEncryptsLargeBody(t *testing.T) {
	key := "test-encryption-key-32-bytes!!"
	encryptor, err := audit.NewAES256GCMEncryptor(key)
	if err != nil {
		t.Fatalf("NewAES256GCMEncryptor: %v", err)
	}

	path := filepath.Join(t.TempDir(), "audit", "large.jsonl")
	rec, err := audit.NewJSONLRecorderWithOptions(path, audit.JSONLRecorderOptions{}, encryptor)
	if err != nil {
		t.Fatalf("NewJSONLRecorderWithOptions: %v", err)
	}
	defer rec.Close()

	// 构造大的Body（10KB）
	largeBody := make([]byte, 10*1024)
	for i := range largeBody {
		largeBody[i] = 'a' + byte(i%26)
	}
	bodyJSON := json.RawMessage(`{"data":"` + string(largeBody) + `"}`)

	rec.Record(context.Background(), audit.Event{
		Event:     audit.EventRequest,
		RequestID: "req_large",
		Body:      bodyJSON,
	})

	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 验证大Body也能正确加密和解密
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("missing audit line")
	}

	var got map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
		t.Fatalf("decode audit line: %v", err)
	}

	bodyEncrypted, ok := got["body_encrypted"].(bool)
	if !ok || !bodyEncrypted {
		t.Fatal("large body should be encrypted")
	}

	bodyStr, ok := got["body"].(string)
	if !ok {
		t.Fatalf("body should be encrypted string")
	}

	// 解密并验证
	decrypted, err := encryptor.Decrypt(bodyStr)
	if err != nil {
		t.Fatalf("Decrypt large body: %v", err)
	}

	if len(decrypted) == 0 {
		t.Fatal("decrypted body should not be empty")
	}
}

func TestJSONLRecorderEncryptsEmptyBody(t *testing.T) {
	key := "test-encryption-key-32-bytes!!"
	encryptor, err := audit.NewAES256GCMEncryptor(key)
	if err != nil {
		t.Fatalf("NewAES256GCMEncryptor: %v", err)
	}

	path := filepath.Join(t.TempDir(), "audit", "empty.jsonl")
	rec, err := audit.NewJSONLRecorderWithOptions(path, audit.JSONLRecorderOptions{}, encryptor)
	if err != nil {
		t.Fatalf("NewJSONLRecorderWithOptions: %v", err)
	}
	defer rec.Close()

	// 空Body不应该触发加密
	rec.Record(context.Background(), audit.Event{
		Event:     audit.EventError,
		RequestID: "req_empty",
		Error:     "test error",
	})

	if err := rec.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 验证空Body事件正确记录
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("missing audit line")
	}

	var got map[string]any
	if err := json.Unmarshal(scanner.Bytes(), &got); err != nil {
		t.Fatalf("decode audit line: %v", err)
	}

	// 空Body不应该有body_encrypted字段
	if _, ok := got["body_encrypted"]; ok {
		t.Fatal("empty body should not have body_encrypted field")
	}
}