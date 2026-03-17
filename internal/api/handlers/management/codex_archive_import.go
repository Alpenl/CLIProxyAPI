package management

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

type CodexArchiveImportSummary struct {
	Imported int                    `json:"imported"`
	Skipped  int                    `json:"skipped"`
	Results  []codexOperationResult `json:"results"`
}

func (h *Handler) ImportCodexArchiveBytes(ctx context.Context, archive []byte) (CodexArchiveImportSummary, error) {
	if h == nil || h.cfg == nil {
		return CodexArchiveImportSummary{}, fmt.Errorf("handler not initialized")
	}

	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return CodexArchiveImportSummary{}, fmt.Errorf("invalid archive: %w", err)
	}

	files := make([]*zip.File, 0, len(reader.File))
	for _, file := range reader.File {
		if file == nil || file.FileInfo().IsDir() {
			continue
		}
		name := path.Clean(strings.TrimSpace(file.Name))
		if !strings.HasPrefix(name, "accounts/") {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}
		files = append(files, file)
	}

	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	summary := CodexArchiveImportSummary{
		Results: make([]codexOperationResult, 0, len(files)),
	}
	for _, file := range files {
		rc, err := file.Open()
		if err != nil {
			summary.Skipped++
			summary.Results = append(summary.Results, codexOperationResult{
				Name:   path.Base(file.Name),
				Status: "skipped",
				Reason: err.Error(),
			})
			continue
		}

		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			summary.Skipped++
			summary.Results = append(summary.Results, codexOperationResult{
				Name:   path.Base(file.Name),
				Status: "skipped",
				Reason: err.Error(),
			})
			continue
		}

		result, err := h.importCodexFile(ctx, path.Base(file.Name), data)
		if err != nil {
			result.Status = "skipped"
			result.Reason = err.Error()
			summary.Skipped++
			summary.Results = append(summary.Results, result)
			continue
		}

		summary.Imported++
		summary.Results = append(summary.Results, result)
	}

	return summary, nil
}
