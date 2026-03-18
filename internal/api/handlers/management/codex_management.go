package management

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/fileperm"
	codexexecutor "github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

type codexImportRequest struct {
	Path           string `json:"path"`
	CleanupInvalid *bool  `json:"cleanup_invalid"`
}

type codexCleanupRequest struct {
	Names []string `json:"names"`
}

type codexOperationResult struct {
	Name   string `json:"name"`
	Email  string `json:"email,omitempty"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type codexCleanupSummary struct {
	Removed   int                    `json:"removed"`
	Refreshed int                    `json:"refreshed"`
	Kept      int                    `json:"kept"`
	Results   []codexOperationResult `json:"results"`
}

func (h *Handler) ListCodexAccounts(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"accounts": h.listCodexAccountEntries(),
	})
}

func (h *Handler) ImportCodexDirectory(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}

	var req codexImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	sourceDir := strings.TrimSpace(req.Path)
	if sourceDir == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}
	info, err := os.Stat(sourceDir)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("failed to access directory: %v", err)})
		return
	}
	if !info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path must be a directory"})
		return
	}

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to read directory: %v", err)})
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	results := make([]codexOperationResult, 0, len(entries))
	imported := 0
	skipped := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}
		data, errRead := os.ReadFile(filepath.Join(sourceDir, name))
		if errRead != nil {
			results = append(results, codexOperationResult{
				Name:   name,
				Status: "skipped",
				Reason: errRead.Error(),
			})
			skipped++
			continue
		}
		result, errImport := h.importCodexFile(c.Request.Context(), name, data)
		if errImport != nil {
			result.Status = "skipped"
			result.Reason = errImport.Error()
			results = append(results, result)
			skipped++
			continue
		}
		results = append(results, result)
		imported++
	}

	response := gin.H{
		"imported": imported,
		"skipped":  skipped,
		"results":  results,
		"accounts": h.listCodexAccountEntries(),
	}
	if cleanup, errCleanup := h.maybeCleanupCodex(c.Request.Context(), req.CleanupInvalid); errCleanup == nil {
		response["cleanup"] = cleanup
		response["accounts"] = h.listCodexAccountEntries()
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) ImportCodexFiles(c *gin.Context) {
	if h == nil || h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}

	if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("failed to parse multipart form: %v", err)})
		return
	}

	var files []*multipart.FileHeader
	if form := c.Request.MultipartForm; form != nil {
		files = append(files, form.File["files"]...)
		files = append(files, form.File["file"]...)
	}
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no files uploaded"})
		return
	}

	results := make([]codexOperationResult, 0, len(files))
	imported := 0
	skipped := 0
	for _, fileHeader := range files {
		if fileHeader == nil {
			continue
		}
		name := filepath.Base(fileHeader.Filename)
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			results = append(results, codexOperationResult{
				Name:   name,
				Status: "skipped",
				Reason: "file must be .json",
			})
			skipped++
			continue
		}
		file, errOpen := fileHeader.Open()
		if errOpen != nil {
			results = append(results, codexOperationResult{
				Name:   name,
				Status: "skipped",
				Reason: errOpen.Error(),
			})
			skipped++
			continue
		}
		data, errRead := io.ReadAll(file)
		_ = file.Close()
		if errRead != nil {
			results = append(results, codexOperationResult{
				Name:   name,
				Status: "skipped",
				Reason: errRead.Error(),
			})
			skipped++
			continue
		}
		result, errImport := h.importCodexFile(c.Request.Context(), name, data)
		if errImport != nil {
			result.Status = "skipped"
			result.Reason = errImport.Error()
			results = append(results, result)
			skipped++
			continue
		}
		results = append(results, result)
		imported++
	}

	var cleanupFlag *bool
	if raw := strings.TrimSpace(c.PostForm("cleanup_invalid")); raw != "" {
		enabled := raw == "1" || strings.EqualFold(raw, "true") || strings.EqualFold(raw, "yes") || strings.EqualFold(raw, "on")
		cleanupFlag = &enabled
	}

	response := gin.H{
		"imported": imported,
		"skipped":  skipped,
		"results":  results,
		"accounts": h.listCodexAccountEntries(),
	}
	if cleanup, errCleanup := h.maybeCleanupCodex(c.Request.Context(), cleanupFlag); errCleanup == nil {
		response["cleanup"] = cleanup
		response["accounts"] = h.listCodexAccountEntries()
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) CleanupInvalidCodexAccounts(c *gin.Context) {
	var req codexCleanupRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
	}

	names := make(map[string]struct{}, len(req.Names))
	for _, name := range req.Names {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			names[trimmed] = struct{}{}
		}
	}

	summary, err := h.cleanupCodexAccounts(c.Request.Context(), names)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"removed":   summary.Removed,
		"refreshed": summary.Refreshed,
		"kept":      summary.Kept,
		"results":   summary.Results,
		"accounts":  h.listCodexAccountEntries(),
	})
}

func (h *Handler) DeleteCodexAccount(c *gin.Context) {
	name := filepath.Base(strings.TrimSpace(c.Param("name")))
	if name == "" || !strings.HasSuffix(strings.ToLower(name), ".json") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid auth file name"})
		return
	}
	if err := h.deleteCodexAccount(c.Request.Context(), name); err != nil {
		status := http.StatusInternalServerError
		if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) listCodexAccountEntries() []gin.H {
	if h == nil || h.cfg == nil {
		return nil
	}
	if cached, ok := h.loadCodexAccountsCache(); ok {
		return cached
	}

	var accounts []gin.H
	if h.authManager == nil {
		accounts = h.listCodexAccountsFromDisk()
	} else {
		auths := h.authManager.List()
		accounts = make([]gin.H, 0, len(auths))
		for _, auth := range auths {
			if auth == nil || !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
				continue
			}
			entry := h.buildAuthFileEntry(auth)
			if entry == nil {
				continue
			}
			accounts = append(accounts, entry)
		}
		sort.Slice(accounts, func(i, j int) bool {
			nameI, _ := accounts[i]["name"].(string)
			nameJ, _ := accounts[j]["name"].(string)
			return strings.ToLower(nameI) < strings.ToLower(nameJ)
		})
	}

	h.storeCodexAccountsCache(accounts)
	return accounts
}

func (h *Handler) listCodexAccountsFromDisk() []gin.H {
	entries, err := os.ReadDir(h.cfg.AuthDir)
	if err != nil {
		return nil
	}
	accounts := make([]gin.H, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}
		fullPath := filepath.Join(h.cfg.AuthDir, name)
		data, errRead := os.ReadFile(fullPath)
		if errRead != nil {
			continue
		}
		var metadata map[string]any
		if err = json.Unmarshal(data, &metadata); err != nil {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(stringMetadata(metadata, "type")), "codex") {
			continue
		}
		info, errInfo := os.Stat(fullPath)
		if errInfo != nil {
			continue
		}
		account := gin.H{
			"name":    name,
			"type":    "codex",
			"email":   stringMetadata(metadata, "email"),
			"path":    fullPath,
			"size":    info.Size(),
			"modtime": info.ModTime(),
			"source":  "file",
		}
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool {
		nameI, _ := accounts[i]["name"].(string)
		nameJ, _ := accounts[j]["name"].(string)
		return strings.ToLower(nameI) < strings.ToLower(nameJ)
	})
	return accounts
}

func (h *Handler) importCodexFile(ctx context.Context, sourceName string, data []byte) (codexOperationResult, error) {
	metadata, email, err := parseCodexMetadata(data)
	result := codexOperationResult{
		Name:  filepath.Base(strings.TrimSpace(sourceName)),
		Email: email,
	}
	if err != nil {
		if result.Name == "" {
			result.Name = "unknown.json"
		}
		return result, err
	}

	fileName := filepath.Base(strings.TrimSpace(sourceName))
	if fileName == "" {
		fileName = fallbackCodexFileName(email)
	}
	if !strings.HasSuffix(strings.ToLower(fileName), ".json") {
		fileName += ".json"
	}
	if reason, duplicate := h.codexDuplicateReason(data, metadata); duplicate {
		result.Name = fileName
		result.Status = "skipped"
		result.Reason = reason
		return result, fmt.Errorf("%s", reason)
	}
	dst := filepath.Join(h.cfg.AuthDir, fileName)
	dir := filepath.Dir(dst)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return result, err
	}
	if err = fileperm.BestEffortChmod(dir, 0o755); err != nil {
		return result, err
	}
	if err = os.WriteFile(dst, data, 0o644); err != nil {
		return result, err
	}
	if err = fileperm.BestEffortChmod(dst, 0o644); err != nil {
		return result, err
	}
	if err = h.registerAuthFromFile(ctx, dst, data); err != nil {
		return result, err
	}

	metadata["type"] = "codex"
	result.Name = fileName
	result.Email = stringMetadata(metadata, "email")
	result.Status = "imported"
	return result, nil
}

func parseCodexMetadata(data []byte) (map[string]any, string, error) {
	metadata := make(map[string]any)
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, "", fmt.Errorf("invalid JSON: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(stringMetadata(metadata, "type")), "codex") {
		return metadata, stringMetadata(metadata, "email"), fmt.Errorf("only codex auth files are supported")
	}
	return metadata, stringMetadata(metadata, "email"), nil
}

func stringMetadata(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func fallbackCodexFileName(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return "codex-imported.json"
	}
	return "codex-" + email + ".json"
}

func (h *Handler) maybeCleanupCodex(ctx context.Context, flag *bool) (codexCleanupSummary, error) {
	if flag != nil && !*flag {
		return codexCleanupSummary{}, nil
	}
	return h.cleanupCodexAccounts(ctx, nil)
}

func (h *Handler) cleanupCodexAccounts(ctx context.Context, names map[string]struct{}) (codexCleanupSummary, error) {
	if h == nil || h.authManager == nil {
		return codexCleanupSummary{}, fmt.Errorf("core auth manager unavailable")
	}

	auths := h.authManager.List()
	sort.Slice(auths, func(i, j int) bool {
		return strings.ToLower(auths[i].FileName) < strings.ToLower(auths[j].FileName)
	})

	summary := codexCleanupSummary{
		Results: make([]codexOperationResult, 0, len(auths)),
	}

	for _, auth := range auths {
		if auth == nil || !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
			continue
		}
		name := strings.TrimSpace(auth.FileName)
		if name == "" {
			name = auth.ID
		}
		if len(names) > 0 {
			if _, ok := names[name]; !ok {
				continue
			}
		}

		result := codexOperationResult{
			Name:  name,
			Email: authEmail(auth),
		}
		path := strings.TrimSpace(authAttribute(auth, "path"))
		if path == "" && h.cfg != nil && strings.TrimSpace(auth.FileName) != "" {
			path = filepath.Join(h.cfg.AuthDir, auth.FileName)
		}
		if path == "" {
			result.Status = "removed"
			result.Reason = "missing auth file path"
			if err := h.removeCodexAuth(ctx, auth, path); err != nil {
				result.Status = "error"
				result.Reason = err.Error()
				summary.Results = append(summary.Results, result)
				continue
			}
			summary.Removed++
			summary.Results = append(summary.Results, result)
			continue
		}

		refreshToken := ""
		if auth.Metadata != nil {
			if token, ok := auth.Metadata["refresh_token"].(string); ok {
				refreshToken = strings.TrimSpace(token)
			}
		}
		if refreshToken == "" {
			result.Status = "removed"
			result.Reason = "missing refresh token"
			if err := h.removeCodexAuth(ctx, auth, path); err != nil {
				result.Status = "error"
				result.Reason = err.Error()
				summary.Results = append(summary.Results, result)
				continue
			}
			summary.Removed++
			summary.Results = append(summary.Results, result)
			continue
		}

		refreshed, err := h.refreshCodexAuth(ctx, auth)
		if err != nil {
			result.Status = "removed"
			result.Reason = err.Error()
			if errRemove := h.removeCodexAuth(ctx, auth, path); errRemove != nil {
				result.Status = "error"
				result.Reason = errRemove.Error()
				summary.Results = append(summary.Results, result)
				continue
			}
			summary.Removed++
			summary.Results = append(summary.Results, result)
			continue
		}

		if err = h.persistCodexAuth(ctx, refreshed, path); err != nil {
			result.Status = "error"
			result.Reason = err.Error()
			summary.Results = append(summary.Results, result)
			continue
		}

		result.Status = "refreshed"
		result.Email = authEmail(refreshed)
		summary.Refreshed++
		summary.Kept++
		summary.Results = append(summary.Results, result)
	}

	return summary, nil
}

func (h *Handler) refreshCodexAuth(ctx context.Context, auth *coreauth.Auth) (*coreauth.Auth, error) {
	if h == nil {
		return nil, fmt.Errorf("handler not initialized")
	}
	if h.codexRefresher != nil {
		return h.codexRefresher(ctx, auth.Clone())
	}
	refresher := codexexecutor.NewCodexExecutor(h.cfg)
	return refresher.Refresh(ctx, auth.Clone())
}

func (h *Handler) persistCodexAuth(ctx context.Context, auth *coreauth.Auth, path string) error {
	if auth == nil {
		return fmt.Errorf("auth is nil")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("auth path is empty")
	}
	if auth.Metadata == nil {
		auth.Metadata = make(map[string]any)
	}
	auth.Metadata["type"] = "codex"

	data, err := json.Marshal(auth.Metadata)
	if err != nil {
		return fmt.Errorf("failed to encode auth metadata: %w", err)
	}
	if err = os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to persist auth file: %w", err)
	}
	if err = fileperm.BestEffortChmod(path, 0o644); err != nil {
		return fmt.Errorf("failed to persist auth file: %w", err)
	}
	return h.registerAuthFromFile(ctx, path, data)
}

func (h *Handler) deleteCodexAccount(ctx context.Context, name string) error {
	if h == nil || h.cfg == nil {
		return fmt.Errorf("handler not initialized")
	}
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" {
		return fmt.Errorf("invalid auth file name")
	}
	targetPath := filepath.Join(h.cfg.AuthDir, name)
	targetAuth := h.findAuthForDelete(name)
	if targetAuth != nil {
		if path := strings.TrimSpace(authAttribute(targetAuth, "path")); path != "" {
			targetPath = path
		}
	}
	if err := os.Remove(targetPath); err != nil {
		return err
	}
	h.invalidateCodexAccountsCache()
	if err := h.deleteTokenRecord(ctx, targetPath); err != nil {
		return err
	}
	if targetAuth != nil {
		h.removeAuthRuntime(targetAuth.ID)
		return nil
	}
	h.removeAuthRuntime(h.authIDForPath(targetPath))
	return nil
}

func (h *Handler) removeCodexAuth(ctx context.Context, auth *coreauth.Auth, path string) error {
	path = strings.TrimSpace(path)
	removedFromDisk := false
	if path != "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		removedFromDisk = true
		_ = h.deleteTokenRecord(ctx, path)
	}
	if removedFromDisk {
		h.invalidateCodexAccountsCache()
	}
	if auth != nil {
		h.removeAuthRuntime(auth.ID)
		return nil
	}
	if path != "" {
		h.removeAuthRuntime(h.authIDForPath(path))
	}
	return nil
}

func (h *Handler) removeAuthRuntime(id string) {
	if h == nil {
		return
	}
	h.invalidateCodexAccountsCache()
	if h.authManager == nil {
		return
	}
	h.authManager.Remove(id)
}
