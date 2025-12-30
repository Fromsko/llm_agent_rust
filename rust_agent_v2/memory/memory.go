package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Memory 记忆接口
type Memory interface {
	Store(ctx context.Context, key string, value any) error
	Retrieve(ctx context.Context, key string) (any, error)
	Search(ctx context.Context, query string) ([]MemoryItem, error)
	Delete(ctx context.Context, key string) error
}

// MemoryItem 记忆项
type MemoryItem struct {
	Key       string    `json:"key"`
	Value     any       `json:"value"`
	Timestamp time.Time `json:"timestamp"`
	Tags      []string  `json:"tags"`
}

// InMemoryStore 内存存储
type InMemoryStore struct {
	mu    sync.RWMutex
	items map[string]*MemoryItem
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{items: make(map[string]*MemoryItem)}
}

func (m *InMemoryStore) Store(ctx context.Context, key string, value any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = &MemoryItem{Key: key, Value: value, Timestamp: time.Now()}
	return nil
}

func (m *InMemoryStore) Retrieve(ctx context.Context, key string) (any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if item, ok := m.items[key]; ok {
		return item.Value, nil
	}
	return nil, nil
}

func (m *InMemoryStore) Search(ctx context.Context, query string) ([]MemoryItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []MemoryItem
	for _, item := range m.items {
		results = append(results, *item)
	}
	return results, nil
}

func (m *InMemoryStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
	return nil
}

// FileStore 文件存储
type FileStore struct {
	dir string
	mu  sync.RWMutex
}

func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &FileStore{dir: dir}, nil
}

func (f *FileStore) Store(ctx context.Context, key string, value any) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	item := &MemoryItem{Key: key, Value: value, Timestamp: time.Now()}
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(f.dir, key+".json"), data, 0644)
}

func (f *FileStore) Retrieve(ctx context.Context, key string) (any, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	data, err := os.ReadFile(filepath.Join(f.dir, key+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var item MemoryItem
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, err
	}

	return item.Value, nil
}

func (f *FileStore) Search(ctx context.Context, query string) ([]MemoryItem, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var results []MemoryItem
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(f.dir, entry.Name()))
		if err != nil {
			continue
		}

		var item MemoryItem
		if err := json.Unmarshal(data, &item); err != nil {
			continue
		}

		results = append(results, item)
	}

	return results, nil
}

func (f *FileStore) Delete(ctx context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return os.Remove(filepath.Join(f.dir, key+".json"))
}

// ProjectContext 项目上下文记忆
type ProjectContext struct {
	ProjectDir   string            `json:"project_dir"`
	CargoToml    string            `json:"cargo_toml"`
	Dependencies map[string]string `json:"dependencies"`
	SourceFiles  []string          `json:"source_files"`
	LastErrors   []string          `json:"last_errors"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// ErrorHistory 错误历史记忆
type ErrorHistory struct {
	Errors []ErrorRecord `json:"errors"`
}

type ErrorRecord struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	File      string    `json:"file"`
	Line      int       `json:"line"`
	Fixed     bool      `json:"fixed"`
	Solution  string    `json:"solution"`
	Timestamp time.Time `json:"timestamp"`
}

func (h *ErrorHistory) Add(record ErrorRecord) {
	record.Timestamp = time.Now()
	h.Errors = append(h.Errors, record)
	// 保留最近 100 条
	if len(h.Errors) > 100 {
		h.Errors = h.Errors[len(h.Errors)-100:]
	}
}

func (h *ErrorHistory) FindSimilar(code string) []ErrorRecord {
	var similar []ErrorRecord
	for _, e := range h.Errors {
		if e.Code == code && e.Fixed {
			similar = append(similar, e)
		}
	}
	return similar
}
