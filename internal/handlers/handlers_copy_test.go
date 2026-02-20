package handlers

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyConfigFileFromTestdata(t *testing.T) {
	testFiles := []string{"client-eu.json", "client-us.json", "client-asia.json"}
	basePath := filepath.Join("..", "..", "testdata", "xray-configs")

	for _, fileName := range testFiles {
		t.Run(fileName, func(t *testing.T) {
			sourcePath := filepath.Join(basePath, fileName)
			destinationPath := filepath.Join(t.TempDir(), "config.json")

			if err := copyConfigFile(sourcePath, destinationPath); err != nil {
				t.Fatalf("copyConfigFile returned error: %v", err)
			}

			sourceData, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatalf("read source file failed: %v", err)
			}
			destinationData, err := os.ReadFile(destinationPath)
			if err != nil {
				t.Fatalf("read destination file failed: %v", err)
			}

			if !bytes.Equal(sourceData, destinationData) {
				t.Fatalf("copied data mismatch for %s", fileName)
			}
		})
	}
}
