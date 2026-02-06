package docx

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractResult holds the extraction results
type ExtractResult struct {
	TempDir   string            // Temporary directory containing extracted files
	MediaDir  string            // Path to word/media directory
	Images    map[string]string // Map of image filename to full path
	CleanupFn func()            // Function to cleanup temp directory
}

// Extract extracts a docx file to a temporary directory and returns image paths
func Extract(docxPath string) (*ExtractResult, error) {
	tempDir, err := os.MkdirTemp("", "ddx-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanupFn := func() {
		os.RemoveAll(tempDir)
	}

	reader, err := zip.OpenReader(docxPath)
	if err != nil {
		cleanupFn()
		return nil, fmt.Errorf("failed to open docx file: %w", err)
	}
	defer reader.Close()

	images := make(map[string]string)
	mediaDir := ""

	for _, file := range reader.File {
		destPath := filepath.Join(tempDir, file.Name)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				cleanupFn()
				return nil, fmt.Errorf("failed to create directory: %w", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			cleanupFn()
			return nil, fmt.Errorf("failed to create parent directory: %w", err)
		}

		if err := extractFile(file, destPath); err != nil {
			cleanupFn()
			return nil, fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}

		if strings.HasPrefix(file.Name, "word/media/") {
			fileName := filepath.Base(file.Name)
			images[fileName] = destPath
			if mediaDir == "" {
				mediaDir = filepath.Dir(destPath)
			}
		}
	}

	return &ExtractResult{
		TempDir:   tempDir,
		MediaDir:  mediaDir,
		Images:    images,
		CleanupFn: cleanupFn,
	}, nil
}

func extractFile(file *zip.File, destPath string) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, rc)
	return err
}

// GetImageList returns a sorted list of image filenames
func (r *ExtractResult) GetImageList() []string {
	var images []string
	for name := range r.Images {
		images = append(images, name)
	}
	return images
}
