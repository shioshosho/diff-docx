package markdown

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shioshosho/diff-docx/internal/image"
)

// ProcessResult holds the markdown processing result
type ProcessResult struct {
	Content     string   // Processed markdown content
	OutputPath  string   // Path to the processed markdown file
	ImagePaths  []string // List of image paths referenced in the markdown
}

// mimeToExts maps MIME sub-types to file extensions found in word/media/
var mimeToExts = map[string][]string{
	"png":          {".png"},
	"jpeg":         {".jpg", ".jpeg"},
	"gif":          {".gif"},
	"bmp":          {".bmp"},
	"tiff":         {".tiff", ".tif"},
	"webp":         {".webp"},
	"x-emf":        {".emf"},
	"x-wmf":        {".wmf"},
	"svg+xml":      {".svg"},
	"vnd.ms-photo": {".wdp"},
}

// ConvertToMarkdown converts a docx file to markdown using markitdown
func ConvertToMarkdown(docxPath string) (string, error) {
	cmd := exec.Command("markitdown", docxPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("markitdown failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// groupImagesByExt groups extracted images by extension, sorted by filename.
func groupImagesByExt(images map[string]string) map[string][]string {
	groups := make(map[string][]string)
	extNames := make(map[string][]string)
	for name := range images {
		ext := strings.ToLower(filepath.Ext(name))
		extNames[ext] = append(extNames[ext], name)
	}
	for ext, names := range extNames {
		sort.Strings(names)
		for _, name := range names {
			groups[ext] = append(groups[ext], images[name])
		}
	}
	return groups
}

// resolveExt finds the extension group for a MIME sub-type
func resolveExt(mimeSubType string, groups map[string][]string) string {
	exts, ok := mimeToExts[mimeSubType]
	if !ok {
		return ""
	}
	for _, ext := range exts {
		if _, exists := groups[ext]; exists {
			return ext
		}
	}
	return ""
}

// ReplaceBase64Images replaces base64 image references with actual file paths.
// For each MIME type, the N-th occurrence maps to imageN.<ext> in word/media/.
func ReplaceBase64Images(content string, images map[string]string) (string, error) {
	groups := groupImagesByExt(images)
	counters := make(map[string]int)

	var result strings.Builder
	rest := content

	for {
		marker := "](data:image/"
		idx := strings.Index(rest, marker)
		if idx < 0 {
			result.WriteString(rest)
			break
		}

		imgStart := strings.LastIndex(rest[:idx], "![")
		if imgStart < 0 {
			result.WriteString(rest[:idx+len(marker)])
			rest = rest[idx+len(marker):]
			continue
		}

		altText := rest[imgStart+2 : idx]

		dataStart := idx + 2
		closeIdx := strings.Index(rest[dataStart:], ")")
		if closeIdx < 0 {
			result.WriteString(rest)
			break
		}
		closeIdx += dataStart

		result.WriteString(rest[:imgStart])

		dataURI := rest[dataStart:closeIdx]
		mimeSubType := ""
		if semiIdx := strings.Index(dataURI, ";"); semiIdx > 0 {
			prefix := "data:image/"
			if strings.HasPrefix(dataURI, prefix) {
				mimeSubType = dataURI[len(prefix):semiIdx]
			}
		}

		ext := resolveExt(mimeSubType, groups)
		if ext != "" {
			idx := counters[ext]
			if idx < len(groups[ext]) {
				imagePath := groups[ext][idx]
				counters[ext]++
				result.WriteString(fmt.Sprintf("![%s](%s)", altText, imagePath))
				rest = rest[closeIdx+1:]
				continue
			}
		}

		result.WriteString(rest[imgStart : closeIdx+1])
		rest = rest[closeIdx+1:]
	}

	return result.String(), nil
}

// BuildPathMapping creates path normalization maps from image match results.
// For matched (identical content) pairs, both docs map to the same canonical name.
// For different/only-in-one, paths are prefixed with the docx basename to differentiate.
func BuildPathMapping(matchResult *image.MatchResult, doc1Base, doc2Base string) (map1, map2 map[string]string) {
	map1 = make(map[string]string)
	map2 = make(map[string]string)

	// Matched pairs: both map to same canonical name (doc1's name)
	for _, pair := range matchResult.Matched {
		map1[pair.Image1.Path] = pair.Image1.Name
		map2[pair.Image2.Path] = pair.Image1.Name
	}

	// Different pairs: prefix with docx basename
	for _, pair := range matchResult.Different {
		map1[pair.Image1.Path] = doc1Base + "/" + pair.Image1.Name
		map2[pair.Image2.Path] = doc2Base + "/" + pair.Image2.Name
	}

	// Only in one side: prefix with docx basename
	for _, img := range matchResult.OnlyIn1 {
		map1[img.Path] = doc1Base + "/" + img.Name
	}
	for _, img := range matchResult.OnlyIn2 {
		map2[img.Path] = doc2Base + "/" + img.Name
	}

	// Skipped: use plain filename
	for _, img := range matchResult.Skipped {
		map1[img.Path] = img.Name
		map2[img.Path] = img.Name
	}

	return map1, map2
}

// NormalizeForDiff replaces temp image paths in markdown content with
// canonical names for diff comparison.
func NormalizeForDiff(content string, pathMapping map[string]string) string {
	result := content
	for oldPath, newName := range pathMapping {
		result = strings.ReplaceAll(result, oldPath, newName)
	}
	return result
}

// virtualDir returns a CWD-relative path derived from the docx path.
// e.g. docs/filename.docx (CWD=$HOME/proj) -> ./docs/filename
func virtualDir(docxPath string) string {
	absPath, err := filepath.Abs(docxPath)
	if err != nil {
		return "./" + strings.TrimSuffix(docxPath, filepath.Ext(docxPath))
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "./" + strings.TrimSuffix(docxPath, filepath.Ext(docxPath))
	}
	relPath, err := filepath.Rel(cwd, absPath)
	if err != nil {
		return "./" + strings.TrimSuffix(docxPath, filepath.Ext(docxPath))
	}
	dir := strings.TrimSuffix(relPath, filepath.Ext(relPath))
	if !strings.HasPrefix(dir, ".") {
		dir = "./" + dir
	}
	return dir
}

// ProcessMarkdown converts docx to markdown and replaces image references.
// Content keeps temp paths (for internal use like NormalizeForDiff).
// The saved md file has virtual relative paths for readability.
func ProcessMarkdown(docxPath string, images map[string]string, tempDir string) (*ProcessResult, error) {
	content, err := ConvertToMarkdown(docxPath)
	if err != nil {
		return nil, err
	}

	processedContent, err := ReplaceBase64Images(content, images)
	if err != nil {
		return nil, err
	}

	absDocxPath, err := filepath.Abs(docxPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path for %s: %w", docxPath, err)
	}
	baseName := strings.TrimSuffix(filepath.Base(absDocxPath), filepath.Ext(absDocxPath))
	outputPath := filepath.Join(filepath.Dir(absDocxPath), baseName+".md")

	// For the saved file, replace temp paths with virtual relative paths
	vDir := virtualDir(docxPath)
	fileContent := strings.ReplaceAll(processedContent, tempDir, vDir)

	if err := os.WriteFile(outputPath, []byte(fileContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write markdown file: %w", err)
	}

	var imagePaths []string
	for _, path := range images {
		imagePaths = append(imagePaths, path)
	}

	return &ProcessResult{
		Content:    processedContent, // temp paths preserved for NormalizeForDiff
		OutputPath: outputPath,
		ImagePaths: imagePaths,
	}, nil
}
