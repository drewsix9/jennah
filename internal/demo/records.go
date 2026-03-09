package demo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func (p *Processor) readAllInput(ctx context.Context) ([]byte, error) {
	if IsGCSPath(p.config.InputDataPath) {
		data, err := ReadGCSFile(ctx, p.config.InputDataPath)
		if err != nil {
			p.errorHandler.HandleError(ErrFileNotFound, err)
			return nil, err
		}
		return data, nil
	}

	data, err := os.ReadFile(p.config.InputDataPath)
	if err != nil {
		if os.IsNotExist(err) {
			p.errorHandler.HandleError(ErrFileNotFound, err)
			return nil, fmt.Errorf("input file not found: %s", p.config.InputDataPath)
		}
		p.errorHandler.HandleError(ErrPermissionDenied, err)
		return nil, fmt.Errorf("cannot read input file: %w", err)
	}
	return data, nil
}

func extractSemanticRecords(data []byte) []string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil
	}

	// JSON array: each top-level object/value is one record.
	if bytes.HasPrefix(trimmed, []byte("[")) && json.Valid(trimmed) {
		var arr []json.RawMessage
		if err := json.Unmarshal(trimmed, &arr); err == nil && len(arr) > 0 {
			out := make([]string, 0, len(arr))
			for _, r := range arr {
				rec := strings.TrimSpace(string(r))
				if rec != "" {
					out = append(out, rec)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}

	// Single JSON object/value: process as one full record.
	if json.Valid(trimmed) {
		return []string{string(trimmed)}
	}

	// Fallback: one record per non-empty line.
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		rec := strings.TrimSpace(line)
		if rec != "" {
			out = append(out, rec)
		}
	}
	return out
}
