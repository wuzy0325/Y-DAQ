package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")

	defaultData := map[string]any{
		"key1": "value1",
		"key2": float64(42),
	}

	store := NewConfigStore(filePath, defaultData)

	if err := store.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	store2 := NewConfigStore(filePath, map[string]any{})
	if err := store2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
}

func TestConfigStore_Set(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.json")

	store := NewConfigStore(filePath, map[string]any{})

	newData := map[string]any{
		"updated": true,
	}

	if err := store.Set(newData); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	data := store.Get()
	if data["updated"] != true {
		t.Error("data was not updated")
	}
}

func TestConfigManager_LoadAll(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewConfigManager(tmpDir)

	if err := manager.LoadAll(); err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
}

func TestTryFixCorruptedJson(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		expect string
	}{
		{"empty", []byte(""), "{}"},
		{"valid", []byte(`{"key":"val"}`), `{"key":"val"}`},
		{"corrupted", []byte("not json"), "{}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tryFixCorruptedJson(tt.input)
			if string(result) != tt.expect {
				t.Errorf("got %s, want %s", string(result), tt.expect)
			}
		})
	}
}
