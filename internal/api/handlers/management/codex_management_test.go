package management

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func newCodexManagementTestHandler(t *testing.T) (*Handler, *coreauth.Manager, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	cfg := &config.Config{
		AuthDir: authDir,
		Port:    8317,
	}
	manager := coreauth.NewManager(nil, nil, nil)
	handler := NewHandlerWithoutConfigFilePath(cfg, manager)
	handler.tokenStore = &memoryAuthStore{}

	return handler, manager, authDir
}

func newCodexManagementRouter(h *Handler) *gin.Engine {
	router := gin.New()
	router.GET("/v0/management/codex/accounts", h.ListCodexAccounts)
	router.POST("/v0/management/codex/import-directory", h.ImportCodexDirectory)
	router.POST("/v0/management/codex/import-files", h.ImportCodexFiles)
	router.POST("/v0/management/codex/cleanup-invalid", h.CleanupInvalidCodexAccounts)
	router.DELETE("/v0/management/codex/accounts/:name", h.DeleteCodexAccount)
	return router
}

func writeAuthJSONFile(t *testing.T, dir string, name string, payload string) string {
	t.Helper()

	fullPath := filepath.Join(dir, name)
	if err := os.WriteFile(fullPath, []byte(payload), 0o600); err != nil {
		t.Fatalf("failed to write auth file %s: %v", fullPath, err)
	}
	return fullPath
}

func registerAuthFile(t *testing.T, h *Handler, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read auth file %s: %v", path, err)
	}
	if err = h.registerAuthFromFile(context.Background(), path, data); err != nil {
		t.Fatalf("failed to register auth file %s: %v", path, err)
	}
}

func decodeJSONBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	return payload
}

func TestCodexManagementListAccounts_FiltersNonCodexProviders(t *testing.T) {
	handler, _, authDir := newCodexManagementTestHandler(t)

	codexPath := writeAuthJSONFile(t, authDir, "codex-alpha.json", `{"type":"codex","email":"alpha@example.com","refresh_token":"refresh-alpha"}`)
	otherPath := writeAuthJSONFile(t, authDir, "other-beta.json", `{"type":"other","email":"beta@example.com"}`)
	registerAuthFile(t, handler, codexPath)
	registerAuthFile(t, handler, otherPath)

	router := newCodexManagementRouter(handler)
	req := httptest.NewRequest(http.MethodGet, "/v0/management/codex/accounts", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	payload := decodeJSONBody(t, rr)
	accountsRaw, ok := payload["accounts"].([]any)
	if !ok {
		t.Fatalf("expected accounts array, payload=%#v", payload)
	}
	if len(accountsRaw) != 1 {
		t.Fatalf("expected 1 codex account, got %d", len(accountsRaw))
	}
	account, ok := accountsRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("expected account object, got %#v", accountsRaw[0])
	}
	if got := account["name"]; got != "codex-alpha.json" {
		t.Fatalf("expected codex account name, got %#v", got)
	}
}

func TestCodexManagementImportDirectory_ImportsValidCodexFiles(t *testing.T) {
	handler, manager, authDir := newCodexManagementTestHandler(t)

	sourceDir := t.TempDir()
	writeAuthJSONFile(t, sourceDir, "codex-first.json", `{"type":"codex","email":"first@example.com","refresh_token":"refresh-first","access_token":"access-first"}`)
	writeAuthJSONFile(t, sourceDir, "other-second.json", `{"type":"other","email":"second@example.com"}`)
	writeAuthJSONFile(t, sourceDir, "broken.json", `{`)

	router := newCodexManagementRouter(handler)
	body := bytes.NewBufferString(`{"path":"` + sourceDir + `","cleanup_invalid":false}`)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/codex/import-directory", body)
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	payload := decodeJSONBody(t, rr)
	if got := int(payload["imported"].(float64)); got != 1 {
		t.Fatalf("expected 1 imported account, got %d", got)
	}
	if got := int(payload["skipped"].(float64)); got != 2 {
		t.Fatalf("expected 2 skipped files, got %d", got)
	}

	importedPath := filepath.Join(authDir, "codex-first.json")
	info, err := os.Stat(importedPath)
	if err != nil {
		t.Fatalf("expected imported file at %s: %v", importedPath, err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("imported file mode = %04o, want 0644", got)
	}

	auths := manager.List()
	if len(auths) != 1 {
		t.Fatalf("expected 1 registered auth, got %d", len(auths))
	}
	if got := auths[0].Provider; got != "codex" {
		t.Fatalf("expected registered auth type codex, got %s", got)
	}
}

func TestCodexManagementImportFiles_ImportsUploadedCodexFiles(t *testing.T) {
	handler, manager, authDir := newCodexManagementTestHandler(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("cleanup_invalid", "false"); err != nil {
		t.Fatalf("failed to write cleanup flag: %v", err)
	}

	fileWriter, err := writer.CreateFormFile("files", "codex-upload.json")
	if err != nil {
		t.Fatalf("failed to create multipart file: %v", err)
	}
	if _, err = fileWriter.Write([]byte(`{"type":"codex","email":"upload@example.com","refresh_token":"refresh-upload","access_token":"access-upload"}`)); err != nil {
		t.Fatalf("failed to write multipart file: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	router := newCodexManagementRouter(handler)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/codex/import-files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	payload := decodeJSONBody(t, rr)
	if got := int(payload["imported"].(float64)); got != 1 {
		t.Fatalf("expected 1 imported account, got %d", got)
	}

	importedPath := filepath.Join(authDir, "codex-upload.json")
	info, err := os.Stat(importedPath)
	if err != nil {
		t.Fatalf("expected imported file at %s: %v", importedPath, err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("uploaded file mode = %04o, want 0644", got)
	}

	auths := manager.List()
	if len(auths) != 1 {
		t.Fatalf("expected 1 registered auth, got %d", len(auths))
	}
	if got := auths[0].FileName; got != "codex-upload.json" {
		t.Fatalf("expected uploaded auth file name codex-upload.json, got %s", got)
	}
}

func TestListCodexAccountsUsesShortTTLCache(t *testing.T) {
	handler, _, authDir := newCodexManagementTestHandler(t)

	authPath := writeAuthJSONFile(t, authDir, "codex-cache.json", `{"type":"codex","email":"cache@example.com","refresh_token":"refresh-cache"}`)
	registerAuthFile(t, handler, authPath)

	router := newCodexManagementRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/v0/management/codex/accounts", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	firstPayload := decodeJSONBody(t, rr)
	firstAccounts, ok := firstPayload["accounts"].([]any)
	if !ok || len(firstAccounts) != 1 {
		t.Fatalf("expected first accounts payload with one entry, got %#v", firstPayload["accounts"])
	}
	firstAccount, ok := firstAccounts[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first account object, got %#v", firstAccounts[0])
	}
	firstSize := int(firstAccount["size"].(float64))

	if err := os.WriteFile(authPath, []byte(`{"type":"codex","email":"cache@example.com","refresh_token":"refresh-cache","access_token":"expanded-access-token"}`), 0o600); err != nil {
		t.Fatalf("failed to mutate auth file: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/v0/management/codex/accounts", nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("second request status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	secondPayload := decodeJSONBody(t, rr)
	secondAccounts, ok := secondPayload["accounts"].([]any)
	if !ok || len(secondAccounts) != 1 {
		t.Fatalf("expected second accounts payload with one entry, got %#v", secondPayload["accounts"])
	}
	secondAccount, ok := secondAccounts[0].(map[string]any)
	if !ok {
		t.Fatalf("expected second account object, got %#v", secondAccounts[0])
	}
	secondSize := int(secondAccount["size"].(float64))

	if secondSize != firstSize {
		t.Fatalf("expected cached size %d on immediate second request, got %d", firstSize, secondSize)
	}
}

func TestCodexManagementCleanupInvalid_RemovesBrokenAccounts(t *testing.T) {
	handler, manager, authDir := newCodexManagementTestHandler(t)

	stalePath := writeAuthJSONFile(t, authDir, "codex-stale.json", `{"type":"codex","email":"stale@example.com","refresh_token":"refresh-stale","access_token":"access-stale"}`)
	validPath := writeAuthJSONFile(t, authDir, "codex-valid.json", `{"type":"codex","email":"valid@example.com","refresh_token":"refresh-valid","access_token":"access-valid"}`)
	registerAuthFile(t, handler, stalePath)
	registerAuthFile(t, handler, validPath)

	handler.codexRefresher = func(_ context.Context, auth *coreauth.Auth) (*coreauth.Auth, error) {
		email, _ := auth.Metadata["email"].(string)
		if email == "stale@example.com" {
			return nil, errors.New("invalid_grant")
		}
		updated := auth.Clone()
		if updated.Metadata == nil {
			updated.Metadata = make(map[string]any)
		}
		updated.Metadata["access_token"] = "access-valid-refreshed"
		updated.Metadata["refresh_token"] = "refresh-valid-refreshed"
		updated.Metadata["last_refresh"] = "2026-03-14T00:00:00Z"
		return updated, nil
	}

	router := newCodexManagementRouter(handler)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/codex/cleanup-invalid", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	payload := decodeJSONBody(t, rr)
	if got := int(payload["removed"].(float64)); got != 1 {
		t.Fatalf("expected 1 removed account, got %d", got)
	}
	if got := int(payload["refreshed"].(float64)); got != 1 {
		t.Fatalf("expected 1 refreshed account, got %d", got)
	}

	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale auth file to be removed, stat err: %v", err)
	}

	validData, err := os.ReadFile(validPath)
	if err != nil {
		t.Fatalf("failed to read refreshed auth file: %v", err)
	}
	if !bytes.Contains(validData, []byte(`"access_token":"access-valid-refreshed"`)) {
		t.Fatalf("expected refreshed auth file to contain updated access token: %s", string(validData))
	}

	auths := manager.List()
	if len(auths) != 1 {
		t.Fatalf("expected 1 remaining auth after cleanup, got %d", len(auths))
	}
	if got := auths[0].FileName; got != "codex-valid.json" {
		t.Fatalf("expected remaining auth codex-valid.json, got %s", got)
	}
}

func TestCodexManagementCleanupInvalid_KeepsAccountsOnTransientRefreshFailure(t *testing.T) {
	handler, manager, authDir := newCodexManagementTestHandler(t)

	transientPath := writeAuthJSONFile(t, authDir, "codex-transient.json", `{"type":"codex","email":"transient@example.com","refresh_token":"refresh-transient","access_token":"access-transient"}`)
	registerAuthFile(t, handler, transientPath)

	handler.codexRefresher = func(_ context.Context, auth *coreauth.Auth) (*coreauth.Auth, error) {
		return nil, errors.New("context deadline exceeded")
	}

	router := newCodexManagementRouter(handler)
	req := httptest.NewRequest(http.MethodPost, "/v0/management/codex/cleanup-invalid", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	payload := decodeJSONBody(t, rr)
	if got := int(payload["removed"].(float64)); got != 0 {
		t.Fatalf("expected 0 removed accounts, got %d", got)
	}
	if got := int(payload["kept"].(float64)); got != 1 {
		t.Fatalf("expected 1 kept account, got %d", got)
	}

	if _, err := os.Stat(transientPath); err != nil {
		t.Fatalf("expected transient auth file to remain, stat err: %v", err)
	}

	auths := manager.List()
	if len(auths) != 1 {
		t.Fatalf("expected 1 remaining auth after cleanup, got %d", len(auths))
	}
}

func TestCodexManagementDeleteAccount_RemovesAuthFile(t *testing.T) {
	handler, manager, authDir := newCodexManagementTestHandler(t)

	authPath := writeAuthJSONFile(t, authDir, "codex-delete.json", `{"type":"codex","email":"delete@example.com","refresh_token":"refresh-delete"}`)
	registerAuthFile(t, handler, authPath)

	router := newCodexManagementRouter(handler)
	req := httptest.NewRequest(http.MethodDelete, "/v0/management/codex/accounts/codex-delete.json", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	if _, err := os.Stat(authPath); !os.IsNotExist(err) {
		t.Fatalf("expected auth file to be removed, stat err: %v", err)
	}
	if remaining := manager.List(); len(remaining) != 0 {
		t.Fatalf("expected auth manager to be empty, got %d entries", len(remaining))
	}
}

func TestImportCodexArchiveBytes_ImportsAccountFilesFromZip(t *testing.T) {
	handler, manager, authDir := newCodexManagementTestHandler(t)

	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	accountFile, err := zipWriter.Create("accounts/codex-archive.json")
	if err != nil {
		t.Fatalf("failed to create zip account entry: %v", err)
	}
	if _, err = accountFile.Write([]byte(`{"type":"codex","email":"archive@example.com","refresh_token":"refresh-archive","access_token":"access-archive"}`)); err != nil {
		t.Fatalf("failed to write zip account entry: %v", err)
	}
	manifestFile, err := zipWriter.Create("manifest.json")
	if err != nil {
		t.Fatalf("failed to create manifest entry: %v", err)
	}
	if _, err = manifestFile.Write([]byte(`{"entryNames":["accounts/codex-archive.json"]}`)); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	if err = zipWriter.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	summary, err := handler.ImportCodexArchiveBytes(context.Background(), archive.Bytes())
	if err != nil {
		t.Fatalf("ImportCodexArchiveBytes() error = %v", err)
	}
	if summary.Imported != 1 {
		t.Fatalf("Imported = %d, want 1", summary.Imported)
	}
	if summary.Skipped != 0 {
		t.Fatalf("Skipped = %d, want 0", summary.Skipped)
	}

	importedPath := filepath.Join(authDir, "codex-archive.json")
	if _, err = os.Stat(importedPath); err != nil {
		t.Fatalf("expected imported file at %s: %v", importedPath, err)
	}

	auths := manager.List()
	if len(auths) != 1 {
		t.Fatalf("expected 1 registered auth, got %d", len(auths))
	}
	if got := auths[0].FileName; got != "codex-archive.json" {
		t.Fatalf("expected imported auth file name codex-archive.json, got %s", got)
	}
}

func TestImportCodexArchiveBytes_SkipsDuplicateEmail(t *testing.T) {
	handler, manager, authDir := newCodexManagementTestHandler(t)

	existingPath := writeAuthJSONFile(t, authDir, "codex-existing.json", `{"type":"codex","email":"duplicate@example.com","refresh_token":"refresh-existing","access_token":"access-existing"}`)
	registerAuthFile(t, handler, existingPath)

	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	accountFile, err := zipWriter.Create("accounts/codex-duplicate.json")
	if err != nil {
		t.Fatalf("failed to create zip account entry: %v", err)
	}
	if _, err = accountFile.Write([]byte(`{"type":"codex","email":"duplicate@example.com","refresh_token":"refresh-new","access_token":"access-new"}`)); err != nil {
		t.Fatalf("failed to write zip account entry: %v", err)
	}
	if err = zipWriter.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	summary, err := handler.ImportCodexArchiveBytes(context.Background(), archive.Bytes())
	if err != nil {
		t.Fatalf("ImportCodexArchiveBytes() error = %v", err)
	}
	if summary.Imported != 0 {
		t.Fatalf("Imported = %d, want 0", summary.Imported)
	}
	if summary.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1", summary.Skipped)
	}

	if _, err := os.Stat(filepath.Join(authDir, "codex-duplicate.json")); !os.IsNotExist(err) {
		t.Fatalf("expected duplicate auth file not to be written, stat err: %v", err)
	}

	auths := manager.List()
	if len(auths) != 1 {
		t.Fatalf("expected 1 registered auth, got %d", len(auths))
	}
	if got := auths[0].FileName; got != "codex-existing.json" {
		t.Fatalf("expected existing auth file name codex-existing.json, got %s", got)
	}
}
