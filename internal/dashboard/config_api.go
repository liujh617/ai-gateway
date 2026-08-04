package dashboard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"open-ai-gateway/internal/config"
)

// ConfigAPI handles configuration management endpoints
type ConfigAPI struct {
	configPath string
	config     *config.Config
	reloadFunc func() error
}

// NewConfigAPI creates a new ConfigAPI instance
func NewConfigAPI(configPath string, cfg *config.Config, reloadFunc func() error) *ConfigAPI {
	return &ConfigAPI{
		configPath: configPath,
		config:     cfg,
		reloadFunc: reloadFunc,
	}
}

// RegisterRoutes registers configuration API routes
func (api *ConfigAPI) RegisterRoutes(r chi.Router) {
	// Provider configuration
	r.Get("/config/providers", api.listProviders)
	r.Post("/config/providers", api.addProvider)
	r.Put("/config/providers/{name}", api.updateProvider)
	r.Delete("/config/providers/{name}", api.deleteProvider)

	// Audit configuration
	r.Get("/config/audit", api.getAuditConfig)
	r.Put("/config/audit", api.updateAuditConfig)
	r.Post("/config/audit/key", api.generateAuditKey)

	// PII configuration
	r.Get("/config/pii", api.getPIIConfig)
	r.Put("/config/pii", api.updatePIIConfig)

	// Content Safety configuration
	r.Get("/config/content-safety", api.getContentSafetyConfig)
	r.Put("/config/content-safety", api.updateContentSafetyConfig)

	// Keywords management
	r.Get("/config/keywords", api.listKeywords)
	r.Post("/config/keywords", api.uploadKeyword)
	r.Delete("/config/keywords/{name}", api.deleteKeyword)
}

// Provider Response structures

type ProviderResponse struct {
	Name            string              `json:"name"`
	BaseURL         string              `json:"base_url"`
	APIKeyMasked    string              `json:"api_key_masked"` // Masked for security
	Enabled         bool                `json:"enabled"`
	Models          []string            `json:"models"`
	DefaultModel    string              `json:"default_model"`
	TimeoutSeconds  int                 `json:"timeout_seconds"`
	MaxRetries      int                 `json:"max_retries"`
	RateLimit       *RateLimitResponse  `json:"rate_limit,omitempty"`
	RequiresRestart bool                `json:"requires_restart"`
}

type RateLimitResponse struct {
	RequestsPerMinute int `json:"requests_per_minute"`
	BurstSize         int `json:"burst_size"`
}

type CreateProviderRequest struct {
	Name           string             `json:"name"`
	BaseURL        string             `json:"base_url"`
	APIKey         string             `json:"api_key"`
	Enabled        bool               `json:"enabled"`
	Models         []string           `json:"models"`
	DefaultModel   string             `json:"default_model"`
	TimeoutSeconds int                `json:"timeout_seconds"`
	MaxRetries     int                `json:"max_retries"`
	RateLimit      *RateLimitRequest  `json:"rate_limit,omitempty"`
}

type UpdateProviderRequest struct {
	BaseURL        string             `json:"base_url"`
	APIKey         string             `json:"api_key,omitempty"` // Optional: omit to keep existing
	Enabled        bool               `json:"enabled"`
	Models         []string           `json:"models"`
	DefaultModel   string             `json:"default_model"`
	TimeoutSeconds int                `json:"timeout_seconds"`
	MaxRetries     int                `json:"max_reries"`
	RateLimit      *RateLimitRequest  `json:"rate_limit,omitempty"`
}

type RateLimitRequest struct {
	RequestsPerMinute int `json:"requests_per_minute"`
	BurstSize         int `json:"burst_size"`
}

// listProviders lists all providers
func (api *ConfigAPI) listProviders(w http.ResponseWriter, r *http.Request) {
	providers := make([]ProviderResponse, 0, len(api.config.Providers))
	
	for name, provider := range api.config.Providers {
		// Mask API key
		apiKeyMasked := maskAPIKey(provider.APIKey)
		
		// Get models for this provider
		models := make([]string, 0)
		for modelName, modelCfg := range api.config.Models {
			if modelCfg.Provider == name {
				models = append(models, modelName)
			}
		}
		
		response := ProviderResponse{
			Name:            name,
			BaseURL:         provider.BaseURL,
			APIKeyMasked:    apiKeyMasked,
			Enabled:         true, // All providers in config are considered enabled
			Models:          models,
			DefaultModel:    provider.DefaultModel,
			TimeoutSeconds:  provider.TimeoutSeconds,
			MaxRetries:      provider.MaxRetries,
			RequiresRestart: false, // Will be set to true if API key changes
		}
		
		if provider.RateLimit.RequestsPerMinute > 0 {
			response.RateLimit = &RateLimitResponse{
				RequestsPerMinute: provider.RateLimit.RequestsPerMinute,
				BurstSize:         provider.RateLimit.BurstSize,
			}
		}
		
		providers = append(providers, response)
	}
	
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"providers": providers,
		"total":     len(providers),
	})
}

// addProvider adds a new provider
func (api *ConfigAPI) addProvider(w http.ResponseWriter, r *http.Request) {
	var req CreateProviderRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	
	// Validate required fields
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Provider name is required", nil)
		return
	}
	if req.BaseURL == "" {
		writeError(w, http.StatusBadRequest, "Base URL is required", nil)
		return
	}
	if req.APIKey == "" {
		writeError(w, http.StatusBadRequest, "API key is required", nil)
		return
	}
	
	// Check if provider already exists
	if _, exists := api.config.Providers[req.Name]; exists {
		writeError(w, http.StatusConflict, "Provider already exists", nil)
		return
	}
	
	// Create provider config
	providerCfg := config.ProviderConfig{
		BaseURL:       req.BaseURL,
		APIKey:        req.APIKey,
		DefaultModel:  req.DefaultModel,
		TimeoutSeconds: req.TimeoutSeconds,
		MaxRetries:    req.MaxRetries,
	}
	
	if req.RateLimit != nil {
		providerCfg.RateLimit = config.RateLimitConfig{
			RequestsPerMinute: req.RateLimit.RequestsPerMinute,
			BurstSize:         req.RateLimit.BurstSize,
		}
	}
	
	// Add to config
	api.config.Providers[req.Name] = providerCfg
	
	// Add models
	for _, modelName := range req.Models {
		api.config.Models[modelName] = config.ModelConfig{
			Provider:        req.Name,
			ModelID:        modelName,
			Enabled:        true,
			TimeoutSeconds: req.TimeoutSeconds,
		}
	}
	
	// Save config
	if err := api.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save configuration", err)
		return
	}
	
	// Return success with restart requirement
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":          "Provider added successfully",
		"provider":         req.Name,
		"requires_restart": true, // New provider requires restart
	})
}

// updateProvider updates an existing provider
func (api *ConfigAPI) updateProvider(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "Provider name is required", nil)
		return
	}
	
	// Check if provider exists
	provider, exists := api.config.Providers[name]
	if !exists {
		writeError(w, http.StatusNotFound, "Provider not found", nil)
		return
	}
	
	var req UpdateProviderRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	
	// Track if restart is required
	requiresRestart := false
	
	// Update fields
	if req.BaseURL != "" {
		provider.BaseURL = req.BaseURL
	}
	
	// API Key: only update if provided
	if req.APIKey != "" {
		provider.APIKey = req.APIKey
		requiresRestart = true // API key change requires restart
	}
	
	provider.DefaultModel = req.DefaultModel
	provider.TimeoutSeconds = req.TimeoutSeconds
	provider.MaxRetries = req.MaxRetries
	
	if req.RateLimit != nil {
		provider.RateLimit = config.RateLimitConfig{
			RequestsPerMinute: req.RateLimit.RequestsPerMinute,
			BurstSize:         req.RateLimit.BurstSize,
		}
	}
	
	// Save back to config
	api.config.Providers[name] = provider
	
	// Update models
	for _, modelName := range req.Models {
		api.config.Models[modelName] = config.ModelConfig{
			Provider:        name,
			ModelID:        modelName,
			Enabled:        true,
			TimeoutSeconds: provider.TimeoutSeconds,
		}
	}
	
	// Save config
	if err := api.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save configuration", err)
		return
	}
	
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":          "Provider updated successfully",
		"provider":         name,
		"requires_restart": requiresRestart,
	})
}

// deleteProvider deletes a provider
func (api *ConfigAPI) deleteProvider(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "Provider name is required", nil)
		return
	}
	
	// Check if provider exists
	if _, exists := api.config.Providers[name]; !exists {
		writeError(w, http.StatusNotFound, "Provider not found", nil)
		return
	}
	
	// Delete provider
	delete(api.config.Providers, name)
	
	// Delete associated models
	for modelName, modelCfg := range api.config.Models {
		if modelCfg.Provider == name {
			delete(api.config.Models, modelName)
		}
	}
	
	// Save config
	if err := api.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save configuration", err)
		return
	}
	
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":          "Provider deleted successfully",
		"provider":         name,
		"requires_restart": true,
	})
}

// Audit Configuration

type AuditConfigResponse struct {
	Enabled          bool   `json:"enabled"`
	Path             string `json:"path"`
	MaxFileBytes     int64  `json:"max_file_bytes"`
	EncryptionEnabled bool  `json:"encryption_enabled"`
	EncryptionKeyPath string `json:"encryption_key_path"`
	LogPII           bool   `json:"log_pii"`
	RedactPII        bool   `json:"redact_pii"`
}

type UpdateAuditConfigRequest struct {
	Enabled          *bool  `json:"enabled,omitempty"`
	Path             string `json:"path,omitempty"`
	MaxFileBytes     *int64 `json:"max_file_bytes,omitempty"`
	EncryptionEnabled *bool  `json:"encryption_enabled,omitempty"`
	LogPII           *bool  `json:"log_pii,omitempty"`
	RedactPII        *bool  `json:"redact_pii,omitempty"`
}

func (api *ConfigAPI) getAuditConfig(w http.ResponseWriter, r *http.Request) {
	response := AuditConfigResponse{
		Enabled:          api.config.Audit.Enabled,
		Path:             api.config.Audit.Path,
		MaxFileBytes:     api.config.Audit.MaxFileBytes,
		EncryptionEnabled: api.config.Audit.Encryption.Enabled,
		EncryptionKeyPath: api.config.Audit.Encryption.KeyPath,
		LogPII:           api.config.PIIDetection.LogPII,
		RedactPII:        api.config.PIIDetection.RedactPII,
	}
	
	writeJSON(w, http.StatusOK, response)
}

func (api *ConfigAPI) updateAuditConfig(w http.ResponseWriter, r *http.Request) {
	var req UpdateAuditConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	
	// Update fields
	if req.Enabled != nil {
		api.config.Audit.Enabled = *req.Enabled
	}
	if req.Path != "" {
		api.config.Audit.Path = req.Path
	}
	if req.MaxFileBytes != nil {
		api.config.Audit.MaxFileBytes = *req.MaxFileBytes
	}
	if req.EncryptionEnabled != nil {
		api.config.Audit.Encryption.Enabled = *req.EncryptionEnabled
	}
	if req.LogPII != nil {
		api.config.PIIDetection.LogPII = *req.LogPII
	}
	if req.RedactPII != nil {
		api.config.PIIDetection.RedactPII = *req.RedactPII
	}
	
	// Save config
	if err := api.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save configuration", err)
		return
	}
	
	// Check if hot reload is possible
	canHotReload := true
	requiresRestart := false
	
	// Encryption changes require restart
	if req.EncryptionEnabled != nil {
		canHotReload = false
		requiresRestart = true
	}
	
	if canHotReload && api.reloadFunc != nil {
		if err := api.reloadFunc(); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to reload configuration", err)
			return
		}
	}
	
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":          "Audit configuration updated successfully",
		"requires_restart": requiresRestart,
	})
}

func (api *ConfigAPI) generateAuditKey(w http.ResponseWriter, r *http.Request) {
	// Generate a new encryption key using the keygen tool
	keyPath := api.config.Audit.Encryption.KeyPath
	if keyPath == "" {
		keyPath = "audit.key"
	}
	
	// Generate key (in production, this would call the keygen tool)
	// For now, return a message indicating manual key generation is needed
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Please use the audit-keygen tool to generate a new key",
		"key_path":      keyPath,
		"command":       fmt.Sprintf("./audit-keygen -output %s", keyPath),
		"requires_restart": true,
	})
}

// PII Configuration

type PIIConfigResponse struct {
	Enabled              bool     `json:"enabled"`
	Action               string   `json:"action"`
	DetectPhoneNumber    bool     `json:"detect_phone_number"`
	DetectIDCard         bool     `json:"detect_id_card"`
	DetectBankCardNumber bool     `json:"detect_bank_card_number"`
	LogPII               bool     `json:"log_pii"`
	RedactPII            bool     `json:"redact_pii"`
}

type UpdatePIIConfigRequest struct {
	Enabled              *bool   `json:"enabled,omitempty"`
	Action               string  `json:"action,omitempty"`
	DetectPhoneNumber    *bool   `json:"detect_phone_number,omitempty"`
	DetectIDCard         *bool   `json:"detect_id_card,omitempty"`
	DetectBankCardNumber *bool   `json:"detect_bank_card_number,omitempty"`
	LogPII               *bool   `json:"log_pii,omitempty"`
	RedactPII            *bool   `json:"redact_pii,omitempty"`
}

func (api *ConfigAPI) getPIIConfig(w http.ResponseWriter, r *http.Request) {
	response := PIIConfigResponse{
		Enabled:              api.config.PIIDetection.Enabled,
		Action:               api.config.PIIDetection.Action,
		DetectPhoneNumber:    api.config.PIIDetection.DetectPhoneNumber,
		DetectIDCard:         api.config.PIIDetection.DetectIDCard,
		DetectBankCardNumber: api.config.PIIDetection.DetectBankCardNumber,
		LogPII:               api.config.PIIDetection.LogPII,
		RedactPII:            api.config.PIIDetection.RedactPII,
	}
	
	writeJSON(w, http.StatusOK, response)
}

func (api *ConfigAPI) updatePIIConfig(w http.ResponseWriter, r *http.Request) {
	var req UpdatePIIConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	
	// Update fields
	if req.Enabled != nil {
		api.config.PIIDetection.Enabled = *req.Enabled
	}
	if req.Action != "" {
		api.config.PIIDetection.Action = req.Action
	}
	if req.DetectPhoneNumber != nil {
		api.config.PIIDetection.DetectPhoneNumber = *req.DetectPhoneNumber
	}
	if req.DetectIDCard != nil {
		api.config.PIIDetection.DetectIDCard = *req.DetectIDCard
	}
	if req.DetectBankCardNumber != nil {
		api.config.PIIDetection.DetectBankCardNumber = *req.DetectBankCardNumber
	}
	if req.LogPII != nil {
		api.config.PIIDetection.LogPII = *req.LogPII
	}
	if req.RedactPII != nil {
		api.config.PIIDetection.RedactPII = *req.RedactPII
	}
	
	// Save config
	if err := api.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save configuration", err)
		return
	}
	
	// Hot reload is supported for PII config
	if api.reloadFunc != nil {
		if err := api.reloadFunc(); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to reload configuration", err)
			return
		}
	}
	
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":          "PII configuration updated successfully",
		"requires_restart": false, // PII config supports hot reload
	})
}

// Content Safety Configuration

type ContentSafetyConfigResponse struct {
	Enabled         bool     `json:"enabled"`
	Action          string   `json:"action"`
	Categories      []string `json:"categories"`
	Threshold       string   `json:"threshold"`
	CustomKeywords  CustomKeywordsResponse `json:"custom_keywords"`
	LogMatches      bool     `json:"log_matches"`
}

type CustomKeywordsResponse struct {
	Enabled bool     `json:"enabled"`
	Paths   []string `json:"paths"`
}

type UpdateContentSafetyConfigRequest struct {
	Enabled    *bool    `json:"enabled,omitempty"`
	Action     string   `json:"action,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Threshold  string   `json:"threshold,omitempty"`
	LogMatches *bool    `json:"log_matches,omitempty"`
}

func (api *ConfigAPI) getContentSafetyConfig(w http.ResponseWriter, r *http.Request) {
	response := ContentSafetyConfigResponse{
		Enabled:    api.config.ContentSafety.Enabled,
		Action:     api.config.ContentSafety.Action,
		Categories: api.config.ContentSafety.Categories,
		Threshold:  api.config.ContentSafety.Threshold,
		CustomKeywords: CustomKeywordsResponse{
			Enabled: api.config.ContentSafety.CustomKeywords.Enabled,
			Paths:   api.config.ContentSafety.CustomKeywords.Paths,
		},
		LogMatches: api.config.ContentSafety.LogMatches,
	}
	
	writeJSON(w, http.StatusOK, response)
}

func (api *ConfigAPI) updateContentSafetyConfig(w http.ResponseWriter, r *http.Request) {
	var req UpdateContentSafetyConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	
	// Update fields
	if req.Enabled != nil {
		api.config.ContentSafety.Enabled = *req.Enabled
	}
	if req.Action != "" {
		api.config.ContentSafety.Action = req.Action
	}
	if len(req.Categories) > 0 {
		api.config.ContentSafety.Categories = req.Categories
	}
	if req.Threshold != "" {
		api.config.ContentSafety.Threshold = req.Threshold
	}
	if req.LogMatches != nil {
		api.config.ContentSafety.LogMatches = *req.LogMatches
	}
	
	// Save config
	if err := api.saveConfig(); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save configuration", err)
		return
	}
	
	// Hot reload is supported for content safety config
	if api.reloadFunc != nil {
		if err := api.reloadFunc(); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to reload configuration", err)
			return
		}
	}
	
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":          "Content safety configuration updated successfully",
		"requires_restart": false, // Content safety config supports hot reload
	})
}

// Keywords Management

type KeywordFileResponse struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Category string `json:"category"`
	Size     int64  `json:"size_bytes"`
	ModTime  string `json:"modified_time"`
}

func (api *ConfigAPI) listKeywords(w http.ResponseWriter, r *http.Request) {
	// List keyword files from the keywords directory
	keywordsDir := "internal/audit/keywords"
	
	files := make([]KeywordFileResponse, 0)
	
	// Walk the keywords directory
	filepath.Walk(keywordsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".txt") {
			// Extract category from filename (e.g., "politics.txt" -> "politics")
			category := strings.TrimSuffix(info.Name(), ".txt")
			
			files = append(files, KeywordFileResponse{
				Name:     info.Name(),
				Path:     path,
				Category: category,
				Size:     info.Size(),
				ModTime:  info.ModTime().Format("2006-01-02T15:04:05Z"),
			})
		}
		
		return nil
	})
	
	// Add custom keyword files from config
	for _, customPath := range api.config.ContentSafety.CustomKeywords.Paths {
		if info, err := os.Stat(customPath); err == nil && !info.IsDir() {
			files = append(files, KeywordFileResponse{
				Name:     filepath.Base(customPath),
				Path:     customPath,
				Category: "custom",
				Size:     info.Size(),
				ModTime:  info.ModTime().Format("2006-01-02T15:04:05Z"),
			})
		}
	}
	
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"keywords": files,
		"total":    len(files),
	})
}

func (api *ConfigAPI) uploadKeyword(w http.ResponseWriter, r *http.Request) {
	// Handle file upload (multipart/form-data)
	// For MVP, we'll use a simpler JSON-based approach
	
	var req struct {
		Category string   `json:"category"`
		Keywords []string `json:"keywords"`
	}
	
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	
	if req.Category == "" || len(req.Keywords) == 0 {
		writeError(w, http.StatusBadRequest, "Category and keywords are required", nil)
		return
	}
	
	// Save to custom keywords file
	filename := fmt.Sprintf("custom_%s.txt", req.Category)
	filepath := filepath.Join("internal/audit/keywords", filename)
	
	// Create file
	file, err := os.Create(filepath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create keyword file", err)
		return
	}
	defer file.Close()
	
	// Write keywords
	for _, keyword := range req.Keywords {
		file.WriteString(keyword + "\n")
	}
	
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":  "Keywords uploaded successfully",
		"filename": filename,
		"path":     filepath,
		"count":    len(req.Keywords),
	})
}

func (api *ConfigAPI) deleteKeyword(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "Keyword filename is required", nil)
		return
	}
	
	// Only allow deleting custom keyword files
	if !strings.HasPrefix(name, "custom_") {
		writeError(w, http.StatusBadRequest, "Can only delete custom keyword files", nil)
		return
	}
	
	filepath := filepath.Join("internal/audit/keywords", name)
	
	// Delete file
	if err := os.Remove(filepath); err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "Keyword file not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete keyword file", err)
		return
	}
	
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "Keyword file deleted successfully",
		"filename": name,
	})
}

// Helper functions

// maskAPIKey masks an API key for secure display
func maskAPIKey(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	
	// Show first 3 and last 4 characters
	if len(apiKey) <= 7 {
		return "****"
	}
	
	return apiKey[:3] + "****" + apiKey[len(apiKey)-4:]
}

// saveConfig saves the configuration to file
func (api *ConfigAPI) saveConfig() error {
	// Read existing config file to preserve formatting and comments
	data, err := os.ReadFile(api.configPath)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	
	// Parse existing config to preserve structure
	var existingConfig map[string]interface{}
	if err := json.Unmarshal(data, &existingConfig); err != nil {
		return fmt.Errorf("parse existing config: %w", err)
	}
	
	// Update with new values
	newData, err := json.MarshalIndent(api.config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	
	// Backup existing config
	backupPath := api.configPath + ".backup"
	if err := os.Rename(api.configPath, backupPath); err != nil {
		return fmt.Errorf("backup config: %w", err)
	}
	
	// Write new config
	if err := os.WriteFile(api.configPath, newData, 0600); err != nil {
		// Restore backup on failure
		os.Rename(backupPath, api.configPath)
		return fmt.Errorf("write config: %w", err)
	}
	
	// Remove backup
	os.Remove(backupPath)
	
	return nil
}

// decodeJSON decodes JSON from request body
func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}