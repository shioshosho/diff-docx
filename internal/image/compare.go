package image

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ImageInfo holds a name and path for an image
type ImageInfo struct {
	Name string // filename e.g. "image1.png"
	Path string // full path e.g. "/tmp/ddx-xxx/word/media/image1.png"
}

// MatchedPair represents two images with identical content
type MatchedPair struct {
	Image1 ImageInfo
	Image2 ImageInfo
}

// DiffPair represents two images with different content
type DiffPair struct {
	Image1   ImageInfo
	Image2   ImageInfo
	PSNR     float64
	DiffPath string // path to generated diff image in diff/imgs/
}

// MatchResult holds the structured result of image set comparison
type MatchResult struct {
	Matched   []MatchedPair
	Different []DiffPair
	OnlyIn1   []ImageInfo
	OnlyIn2   []ImageInfo
	Skipped   []ImageInfo
}

// PSNRThreshold is the threshold below which images are considered different
const PSNRThreshold = 1.0

var rasterExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
	".bmp": true, ".gif": true, ".tiff": true,
	".tif": true, ".webp": true,
}

var vectorExts = map[string]bool{
	".wmf": true, ".emf": true, ".svg": true,
}

var hasLibreOffice = sync.OnceValue(func() bool {
	_, err := exec.LookPath("libreoffice")
	return err == nil
})

func canCompareExt(ext string) bool {
	ext = strings.ToLower(ext)
	if rasterExts[ext] {
		return true
	}
	if vectorExts[ext] {
		return hasLibreOffice()
	}
	return false
}

type imageEntry struct {
	name string
	path string
}

func groupByExt(images map[string]string) map[string][]imageEntry {
	groups := make(map[string][]imageEntry)
	for name, path := range images {
		ext := strings.ToLower(filepath.Ext(name))
		groups[ext] = append(groups[ext], imageEntry{name, path})
	}
	for ext := range groups {
		sort.Slice(groups[ext], func(i, j int) bool {
			return groups[ext][i].name < groups[ext][j].name
		})
	}
	return groups
}

// compare runs ImageMagick compare and returns the result
func compare(image1, image2, outputDir string) (isDifferent bool, psnr float64, diffPath string, err error) {
	baseName := strings.TrimSuffix(filepath.Base(image1), filepath.Ext(image1))
	diffPath = filepath.Join(outputDir, baseName+"_cmp.png")

	cmd := exec.Command("magick", "compare", "-verbose", "-metric", "PSNR", image1, image2, diffPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	output := stderr.String() + stdout.String()

	isDifferent, psnr = parsePSNROutput(output)

	if !isDifferent {
		os.Remove(diffPath)
		diffPath = ""
	}

	if runErr != nil && !isDifferent {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			if exitErr.ExitCode() > 1 {
				return false, -1, "", fmt.Errorf("ImageMagick compare failed: %w\nOutput: %s", runErr, output)
			}
		}
	}

	return isDifferent, psnr, diffPath, nil
}

func parsePSNROutput(output string) (isDifferent bool, psnr float64) {
	channelPattern := regexp.MustCompile(`(?i)(red|green|blue|all):\s*([\d.]+|inf)`)
	matches := channelPattern.FindAllStringSubmatch(output, -1)

	psnr = -1
	for _, match := range matches {
		if len(match) >= 3 {
			value := match[2]
			if strings.ToLower(value) == "inf" {
				continue
			}
			psnrValue, err := strconv.ParseFloat(value, 64)
			if err != nil {
				continue
			}
			if psnr < 0 || psnrValue < psnr {
				psnr = psnrValue
			}
			if psnrValue < PSNRThreshold {
				isDifferent = true
			}
		}
	}

	if psnr < 0 {
		if strings.Contains(output, " 0 ") || strings.Contains(output, " 0\n") {
			isDifferent = true
			psnr = 0
		} else {
			psnr = -1
		}
	}

	return isDifferent, psnr
}

// MatchImageSets compares two image sets using content-based matching and
// outputs diff artifacts to diffImgsDir.
func MatchImageSets(images1, images2 map[string]string, diffImgsDir string) (*MatchResult, error) {
	tempDir, err := os.MkdirTemp("", "ddx-match-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	groups1 := groupByExt(images1)
	groups2 := groupByExt(images2)

	allExts := make(map[string]bool)
	for ext := range groups1 {
		allExts[ext] = true
	}
	for ext := range groups2 {
		allExts[ext] = true
	}
	sortedExts := make([]string, 0, len(allExts))
	for ext := range allExts {
		sortedExts = append(sortedExts, ext)
	}
	sort.Strings(sortedExts)

	result := &MatchResult{}

	for _, ext := range sortedExts {
		list1 := groups1[ext]
		list2 := groups2[ext]

		if !canCompareExt(ext) {
			for _, img := range list1 {
				result.Skipped = append(result.Skipped, ImageInfo{img.name, img.path})
			}
			for _, img := range list2 {
				result.Skipped = append(result.Skipped, ImageInfo{img.name, img.path})
			}
			continue
		}

		if err := matchExtGroup(list1, list2, tempDir, diffImgsDir, result); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func matchExtGroup(list1, list2 []imageEntry, tempDir, diffImgsDir string, result *MatchResult) error {
	matched1 := make(map[int]bool)
	matched2 := make(map[int]bool)

	// Phase 1: find identical pairs by content
	for i, img1 := range list1 {
		for j, img2 := range list2 {
			if matched2[j] {
				continue
			}
			isDiff, _, _, err := compare(img1.path, img2.path, tempDir)
			if err != nil {
				continue
			}
			if !isDiff {
				matched1[i] = true
				matched2[j] = true
				result.Matched = append(result.Matched, MatchedPair{
					Image1: ImageInfo{img1.name, img1.path},
					Image2: ImageInfo{img2.name, img2.path},
				})
				break
			}
		}
	}

	// Collect unmatched
	var unmatched1, unmatched2 []imageEntry
	for i, img := range list1 {
		if !matched1[i] {
			unmatched1 = append(unmatched1, img)
		}
	}
	for j, img := range list2 {
		if !matched2[j] {
			unmatched2 = append(unmatched2, img)
		}
	}

	// Phase 2: pair remaining by order, generate diff images
	minLen := len(unmatched1)
	if len(unmatched2) < minLen {
		minLen = len(unmatched2)
	}
	for i := 0; i < minLen; i++ {
		img1 := unmatched1[i]
		img2 := unmatched2[i]

		isDiff, psnr, tmpDiffPath, err := compare(img1.path, img2.path, diffImgsDir)
		if err != nil {
			return fmt.Errorf("failed to compare %s vs %s: %w", img1.name, img2.name, err)
		}

		// Rename diff image to name1-name2.ext
		finalDiffPath := ""
		if isDiff && tmpDiffPath != "" {
			ext := filepath.Ext(img1.name)
			base1 := strings.TrimSuffix(img1.name, ext)
			base2 := strings.TrimSuffix(img2.name, ext)
			finalDiffPath = filepath.Join(diffImgsDir, base1+"-"+base2+ext)
			os.Rename(tmpDiffPath, finalDiffPath)
		}

		result.Different = append(result.Different, DiffPair{
			Image1:   ImageInfo{img1.name, img1.path},
			Image2:   ImageInfo{img2.name, img2.path},
			PSNR:     psnr,
			DiffPath: finalDiffPath,
		})
	}

	// Phase 3: only in one side
	for i := minLen; i < len(unmatched1); i++ {
		result.OnlyIn1 = append(result.OnlyIn1, ImageInfo{unmatched1[i].name, unmatched1[i].path})
	}
	for i := minLen; i < len(unmatched2); i++ {
		result.OnlyIn2 = append(result.OnlyIn2, ImageInfo{unmatched2[i].name, unmatched2[i].path})
	}

	return nil
}

// CopyFile copies a file from src to dst
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
