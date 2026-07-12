package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Executor runs built-in tools, constrained to a workspace directory.
type Executor struct {
	workspaceDir string
}

func NewExecutor(workspaceDir string) *Executor {
	return &Executor{workspaceDir: workspaceDir}
}

func (e *Executor) Execute(ctx context.Context, toolName string, input json.RawMessage) (json.RawMessage, error) {
	switch toolName {
	case "list_files":
		return e.listFiles(ctx, input)
	case "read_file":
		return e.readFile(ctx, input)
	case "search_code":
		return e.searchCode(ctx, input)
	case "run_tests":
		return e.runTests(ctx, input)
	case "summarize_logs":
		return e.summarizeLogs(ctx, input)
	case "create_patch":
		return e.createPatch(ctx, input)
	case "apply_patch":
		return e.applyPatch(ctx, input)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// safePath resolves and validates a path is within the workspace.
func (e *Executor) safePath(requested string) (string, error) {
	if requested == "" {
		requested = "."
	}
	full := filepath.Join(e.workspaceDir, filepath.Clean(requested))
	rel, err := filepath.Rel(e.workspaceDir, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %q is outside the workspace", requested)
	}
	return full, nil
}

// --- list_files ---

type listFilesInput struct {
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
}

func (e *Executor) listFiles(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var in listFilesInput
	_ = json.Unmarshal(raw, &in)

	dir, err := e.safePath(in.Path)
	if err != nil {
		return nil, err
	}

	var files []string
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && path != dir {
			files = append(files, filepath.ToSlash(strings.TrimPrefix(path, e.workspaceDir+string(os.PathSeparator)))+"/")
			return nil
		}
		if !d.IsDir() {
			rel := filepath.ToSlash(strings.TrimPrefix(path, e.workspaceDir+string(os.PathSeparator)))
			if in.Pattern == "" {
				files = append(files, rel)
			} else {
				matched, _ := filepath.Match(in.Pattern, d.Name())
				if matched {
					files = append(files, rel)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if files == nil {
		files = []string{}
	}

	return json.Marshal(map[string]any{"files": files, "count": len(files), "path": in.Path})
}

// --- read_file ---

type readFileInput struct {
	Path     string `json:"path"`
	MaxBytes int    `json:"maxBytes"`
}

func (e *Executor) readFile(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var in readFileInput
	_ = json.Unmarshal(raw, &in)
	if in.MaxBytes <= 0 {
		in.MaxBytes = 50_000
	}

	full, err := e.safePath(in.Path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	if len(data) > in.MaxBytes {
		data = data[:in.MaxBytes]
	}

	lines := bytes.Count(data, []byte("\n"))
	return json.Marshal(map[string]any{
		"content":    string(data),
		"sizeBytes":  len(data),
		"lines":      lines,
		"truncated":  len(data) == in.MaxBytes,
	})
}

// --- search_code ---

type searchCodeInput struct {
	Pattern     string `json:"pattern"`
	Path        string `json:"path"`
	FilePattern string `json:"filePattern"`
	MaxResults  int    `json:"maxResults"`
}

type searchMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

func (e *Executor) searchCode(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var in searchCodeInput
	_ = json.Unmarshal(raw, &in)
	if in.Pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}
	if in.MaxResults <= 0 {
		in.MaxResults = 50
	}

	dir, err := e.safePath(in.Path)
	if err != nil {
		return nil, err
	}

	var matches []searchMatch
	patternLower := strings.ToLower(in.Pattern)

	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || len(matches) >= in.MaxResults {
			return nil
		}
		if in.FilePattern != "" {
			matched, _ := filepath.Match(in.FilePattern, d.Name())
			if !matched {
				return nil
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		scanner := bufio.NewScanner(bytes.NewReader(data))
		lineNum := 0
		rel := filepath.ToSlash(strings.TrimPrefix(path, e.workspaceDir+string(os.PathSeparator)))
		for scanner.Scan() {
			lineNum++
			if len(matches) >= in.MaxResults {
				break
			}
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), patternLower) {
				matches = append(matches, searchMatch{
					File:    rel,
					Line:    lineNum,
					Content: strings.TrimSpace(line),
				})
			}
		}
		return nil
	})

	if matches == nil {
		matches = []searchMatch{}
	}
	return json.Marshal(map[string]any{"matches": matches, "total": len(matches)})
}

// --- run_tests ---

type runTestsInput struct {
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

func (e *Executor) runTests(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var in runTestsInput
	_ = json.Unmarshal(raw, &in)
	if in.Command == "" {
		in.Command = "go test ./..."
	}
	if in.TimeoutSeconds <= 0 {
		in.TimeoutSeconds = 120
	}

	timeout := time.Duration(in.TimeoutSeconds) * time.Second
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(cmdCtx, "sh", "-c", in.Command)
	cmd.Dir = e.workspaceDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	elapsed := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return json.Marshal(map[string]any{
		"exitCode":   exitCode,
		"stdout":     stdout.String(),
		"stderr":     stderr.String(),
		"durationMs": elapsed,
		"command":    in.Command,
	})
}

// --- summarize_logs ---

type summarizeLogsInput struct {
	Content  string `json:"content"`
	MaxLines int    `json:"maxLines"`
}

func (e *Executor) summarizeLogs(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var in summarizeLogsInput
	_ = json.Unmarshal(raw, &in)
	if in.MaxLines <= 0 {
		in.MaxLines = 500
	}

	var errors, warnings []string
	totalLines := 0

	scanner := bufio.NewScanner(strings.NewReader(in.Content))
	for scanner.Scan() {
		if totalLines >= in.MaxLines {
			break
		}
		totalLines++
		line := scanner.Text()
		lower := strings.ToLower(line)

		switch {
		case strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "panic"):
			errors = append(errors, strings.TrimSpace(line))
		case strings.Contains(lower, "warn"):
			warnings = append(warnings, strings.TrimSpace(line))
		}
	}

	if errors == nil {
		errors = []string{}
	}
	if warnings == nil {
		warnings = []string{}
	}

	summary := fmt.Sprintf("Scanned %d lines. Found %d error(s) and %d warning(s).", totalLines, len(errors), len(warnings))

	return json.Marshal(map[string]any{
		"summary":    summary,
		"errors":     errors,
		"warnings":   warnings,
		"totalLines": totalLines,
	})
}

// --- create_patch ---

type createPatchInput struct {
	FilePath       string `json:"filePath"`
	PatchedContent string `json:"patchedContent"`
}

func (e *Executor) createPatch(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var in createPatchInput
	_ = json.Unmarshal(raw, &in)
	if in.FilePath == "" {
		return nil, fmt.Errorf("filePath is required")
	}

	full, err := e.safePath(in.FilePath)
	if err != nil {
		return nil, err
	}

	original, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("read original file: %w", err)
	}

	// Write both versions to temp files and diff them
	origTmp, err := os.CreateTemp("", "agentops-orig-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(origTmp.Name())
	_, _ = origTmp.Write(original)
	origTmp.Close()

	patchedTmp, err := os.CreateTemp("", "agentops-patched-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(patchedTmp.Name())
	_, _ = patchedTmp.WriteString(in.PatchedContent)
	patchedTmp.Close()

	cmd := exec.CommandContext(ctx, "diff", "-u",
		"--label", "a/"+in.FilePath,
		"--label", "b/"+in.FilePath,
		origTmp.Name(), patchedTmp.Name())
	out, _ := cmd.Output() // diff exits 1 if files differ — that's expected

	return json.Marshal(map[string]any{
		"patch":    string(out),
		"filePath": in.FilePath,
	})
}

// --- apply_patch ---

type applyPatchInput struct {
	Patch  string `json:"patch"`
	DryRun bool   `json:"dryRun"`
}

func (e *Executor) applyPatch(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var in applyPatchInput
	_ = json.Unmarshal(raw, &in)
	if in.Patch == "" {
		return nil, fmt.Errorf("patch is required")
	}

	// Write patch to a temp file
	patchTmp, err := os.CreateTemp("", "agentops-patch-*.patch")
	if err != nil {
		return nil, err
	}
	defer os.Remove(patchTmp.Name())
	_, _ = patchTmp.WriteString(in.Patch)
	patchTmp.Close()

	args := []string{"-p1", "--input", patchTmp.Name()}
	if in.DryRun {
		args = append(args, "--dry-run")
	}

	cmd := exec.CommandContext(ctx, "patch", args...)
	cmd.Dir = e.workspaceDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return json.Marshal(map[string]any{
		"applied":  exitCode == 0,
		"dryRun":   in.DryRun,
		"stdout":   stdout.String(),
		"stderr":   stderr.String(),
		"exitCode": exitCode,
	})
}
