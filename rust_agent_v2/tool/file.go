package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

// FileRead 文件读取工具
type FileRead struct{}

func NewFileRead() *FileRead { return &FileRead{} }

func (t *FileRead) Name() string        { return "file_read" }
func (t *FileRead) Description() string { return "读取文件内容" }
func (t *FileRead) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "文件路径"},
		},
		"required": []string{"path"},
	}
}

func (t *FileRead) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	data, err := os.ReadFile(args.Path)
	if err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}

	return FormatOutput(map[string]any{"success": true, "content": string(data)}), nil
}

// FileWrite 文件写入工具
type FileWrite struct{}

func NewFileWrite() *FileWrite { return &FileWrite{} }

func (t *FileWrite) Name() string        { return "file_write" }
func (t *FileWrite) Description() string { return "写入文件内容" }
func (t *FileWrite) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "文件路径"},
			"content": map[string]any{"type": "string", "description": "文件内容"},
		},
		"required": []string{"path", "content"},
	}
}

func (t *FileWrite) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	// 确保目录存在
	dir := filepath.Dir(args.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}

	if err := os.WriteFile(args.Path, []byte(args.Content), 0644); err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}

	return FormatOutput(map[string]any{"success": true, "path": args.Path}), nil
}

// FileList 文件列表工具
type FileList struct{}

func NewFileList() *FileList { return &FileList{} }

func (t *FileList) Name() string        { return "file_list" }
func (t *FileList) Description() string { return "列出目录下的文件" }
func (t *FileList) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":      map[string]any{"type": "string", "description": "目录路径"},
			"recursive": map[string]any{"type": "boolean", "description": "是否递归"},
		},
		"required": []string{"path"},
	}
}

func (t *FileList) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	var files []string

	if args.Recursive {
		filepath.Walk(args.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				files = append(files, path)
			}
			return nil
		})
	} else {
		entries, err := os.ReadDir(args.Path)
		if err != nil {
			return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
		}
		for _, entry := range entries {
			files = append(files, filepath.Join(args.Path, entry.Name()))
		}
	}

	return FormatOutput(map[string]any{"success": true, "files": files}), nil
}

// FileDelete 文件删除工具
type FileDelete struct{}

func NewFileDelete() *FileDelete { return &FileDelete{} }

func (t *FileDelete) Name() string        { return "file_delete" }
func (t *FileDelete) Description() string { return "删除文件或目录" }
func (t *FileDelete) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "description": "文件或目录路径"},
		},
		"required": []string{"path"},
	}
}

func (t *FileDelete) Run(ctx context.Context, input string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", err
	}

	if err := os.RemoveAll(args.Path); err != nil {
		return FormatOutput(map[string]any{"success": false, "error": err.Error()}), nil
	}

	return FormatOutput(map[string]any{"success": true}), nil
}

// CreateDefaultRegistry 创建默认工具注册表
func CreateDefaultRegistry() *Registry {
	r := NewRegistry()

	// Cargo 工具
	r.Register(NewCargoInit())
	r.Register(NewCargoCheck())
	r.Register(NewCargoBuild())
	r.Register(NewCargoRun())
	r.Register(NewCargoTest())
	r.Register(NewCargoClippy())

	// 文件工具
	r.Register(NewFileRead())
	r.Register(NewFileWrite())
	r.Register(NewFileList())
	r.Register(NewFileDelete())

	return r
}
