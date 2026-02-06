package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shioshosho/diff-docx/internal/diff"
	"github.com/shioshosho/diff-docx/internal/docx"
	"github.com/shioshosho/diff-docx/internal/image"
	"github.com/shioshosho/diff-docx/internal/markdown"
	"github.com/shioshosho/diff-docx/internal/progress"
)

const version = "1.0.0"

func main() {
	showVersion := flag.Bool("version", false, "Show version")
	showHelp := flag.Bool("help", false, "Show help")
	verbose := flag.Bool("verbose", false, "Show verbose output")
	convertPNG := flag.Bool("convert-png", true, "Convert vector images (wmf/emf/svg) to PNG via ImageMagick before comparison")
	flag.BoolVar(showVersion, "v", false, "Show version (shorthand)")
	flag.BoolVar(showHelp, "h", false, "Show help (shorthand)")

	flag.Parse()

	if *showVersion {
		fmt.Printf("ddx version %s\n", version)
		os.Exit(0)
	}

	if *showHelp || flag.NArg() < 2 {
		printUsage()
		os.Exit(0)
	}

	file1 := flag.Arg(0)
	file2 := flag.Arg(1)

	if err := validateInputFiles(file1, file2); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := diff.CheckDependencies(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := runDiff(file1, file2, *verbose, *convertPNG); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("ddx - Docx Diff Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ddx [options] <file1.docx> <file2.docx>")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -h, --help          Show this help message")
	fmt.Println("  -v, --version       Show version")
	fmt.Println("  --verbose           Show verbose output")
	fmt.Println("  --convert-png       Convert vector images (wmf/emf/svg) to PNG before comparison (default: true)")
	fmt.Println("                      Use --convert-png=false to disable and require LibreOffice instead")
	fmt.Println()
	fmt.Println("Output:")
	fmt.Println("  diff/diff.md                        Markdown diff (unified format)")
	fmt.Println("  diff/imgs/<name1>-<name2>.<ext>     Image diff (magick compare)")
	fmt.Println("  diff/imgs/original/<docx>/          Changed original images")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ddx before.docx after.docx")
	fmt.Println()
	fmt.Println("Requirements:")
	fmt.Println("  - markitdown (https://github.com/microsoft/markitdown)")
	fmt.Println("  - delta (https://github.com/dandavison/delta)")
	fmt.Println("  - ImageMagick (magick command)")
}

func validateInputFiles(file1, file2 string) error {
	for _, f := range []string{file1, file2} {
		if !strings.HasSuffix(strings.ToLower(f), ".docx") {
			return fmt.Errorf("file %s is not a .docx file", f)
		}
		if _, err := os.Stat(f); os.IsNotExist(err) {
			return fmt.Errorf("file %s does not exist", f)
		}
	}
	return nil
}

func docxBaseName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func runDiff(file1, file2 string, verbose, convertPNG bool) error {
	doc1Base := docxBaseName(file1)
	doc2Base := docxBaseName(file2)

	bar := progress.New(7)

	// 1. Extract docx files to temp directories
	bar.Advance("Extracting " + filepath.Base(file1) + "...")
	extract1, err := docx.Extract(file1)
	if err != nil {
		bar.Done()
		return fmt.Errorf("failed to extract %s: %w", file1, err)
	}
	defer extract1.CleanupFn()

	bar.Advance("Extracting " + filepath.Base(file2) + "...")
	extract2, err := docx.Extract(file2)
	if err != nil {
		bar.Done()
		return fmt.Errorf("failed to extract %s: %w", file2, err)
	}
	defer extract2.CleanupFn()

	// 2. Create output directory structure
	diffImgsDir := filepath.Join("diff", "imgs")
	orig1Dir := filepath.Join("diff", "imgs", "original", doc1Base)
	orig2Dir := filepath.Join("diff", "imgs", "original", doc2Base)

	for _, dir := range []string{diffImgsDir, orig1Dir, orig2Dir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			bar.Done()
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// 3. Convert to markdown and save alongside docx
	bar.Advance("Converting " + filepath.Base(file1) + " to markdown...")
	md1, err := markdown.ProcessMarkdown(file1, extract1.Images, extract1.TempDir)
	if err != nil {
		bar.Done()
		return fmt.Errorf("failed to process %s: %w", file1, err)
	}

	bar.Advance("Converting " + filepath.Base(file2) + " to markdown...")
	md2, err := markdown.ProcessMarkdown(file2, extract2.Images, extract2.TempDir)
	if err != nil {
		bar.Done()
		return fmt.Errorf("failed to process %s: %w", file2, err)
	}

	// 4. Image matching
	bar.Advance("Matching images...")
	matchResult, err := image.MatchImageSets(extract1.Images, extract2.Images, diffImgsDir, convertPNG)
	if err != nil {
		bar.Done()
		return fmt.Errorf("failed to match images: %w", err)
	}

	// 5. Copy original images for changed pairs
	bar.Advance("Copying original images...")
	if err := copyOriginalImages(matchResult, orig1Dir, orig2Dir); err != nil {
		bar.Done()
		return fmt.Errorf("failed to copy original images: %w", err)
	}

	// 6. Generate diff/diff.md with normalized image paths
	bar.Advance("Generating diff.md...")
	map1, map2 := markdown.BuildPathMapping(matchResult, doc1Base, doc2Base)
	norm1 := markdown.NormalizeForDiff(md1.Content, map1)
	norm2 := markdown.NormalizeForDiff(md2.Content, map2)

	// Write normalized markdown to temp files for diff
	tmpDir, err := os.MkdirTemp("", "ddx-normdiff-*")
	if err != nil {
		bar.Done()
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	normPath1 := filepath.Join(tmpDir, doc1Base+".md")
	normPath2 := filepath.Join(tmpDir, doc2Base+".md")

	if err := os.WriteFile(normPath1, []byte(norm1), 0644); err != nil {
		bar.Done()
		return err
	}
	if err := os.WriteFile(normPath2, []byte(norm2), 0644); err != nil {
		bar.Done()
		return err
	}

	if err := diff.GenerateDiffFile(normPath1, normPath2, filepath.Join("diff", "diff.md")); err != nil {
		bar.Done()
		return fmt.Errorf("failed to generate diff.md: %w", err)
	}

	// 7. Display diff via delta
	bar.Done()

	fmt.Println("=== Markdown Diff ===")
	fmt.Println()
	if err := diff.ShowDiffWithFallback(normPath1, normPath2); err != nil {
		return fmt.Errorf("failed to show diff: %w", err)
	}

	// 8. Print summary
	fmt.Println()
	fmt.Println("=== Image Comparison ===")
	fmt.Println()
	printMatchSummary(matchResult, verbose)

	fmt.Println()
	fmt.Println("=== Output ===")
	fmt.Printf("  diff/diff.md\n")
	if len(matchResult.Different) > 0 {
		fmt.Printf("  diff/imgs/ (%d diff images)\n", len(matchResult.Different))
		fmt.Printf("  diff/imgs/original/%s/\n", doc1Base)
		fmt.Printf("  diff/imgs/original/%s/\n", doc2Base)
	}

	return nil
}

func copyOriginalImages(matchResult *image.MatchResult, orig1Dir, orig2Dir string) error {
	// Copy originals for different pairs
	for _, pair := range matchResult.Different {
		dst1 := filepath.Join(orig1Dir, pair.Image1.Name)
		if err := image.CopyFile(pair.Image1.Path, dst1); err != nil {
			return fmt.Errorf("failed to copy %s: %w", pair.Image1.Name, err)
		}
		dst2 := filepath.Join(orig2Dir, pair.Image2.Name)
		if err := image.CopyFile(pair.Image2.Path, dst2); err != nil {
			return fmt.Errorf("failed to copy %s: %w", pair.Image2.Name, err)
		}
	}

	// Copy originals for only-in-one
	for _, img := range matchResult.OnlyIn1 {
		dst := filepath.Join(orig1Dir, img.Name)
		if err := image.CopyFile(img.Path, dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", img.Name, err)
		}
	}
	for _, img := range matchResult.OnlyIn2 {
		dst := filepath.Join(orig2Dir, img.Name)
		if err := image.CopyFile(img.Path, dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", img.Name, err)
		}
	}

	return nil
}

func printMatchSummary(result *image.MatchResult, verbose bool) {
	if verbose {
		for _, pair := range result.Matched {
			fmt.Printf("  [SAME] %s <-> %s\n", pair.Image1.Name, pair.Image2.Name)
		}
	}

	for _, pair := range result.Different {
		fmt.Printf("  [DIFF] %s <-> %s", pair.Image1.Name, pair.Image2.Name)
		if pair.PSNR >= 0 {
			fmt.Printf(" (PSNR: %.3f)", pair.PSNR)
		}
		fmt.Println()
		if verbose && pair.DiffPath != "" {
			fmt.Printf("         -> %s\n", pair.DiffPath)
		}
	}

	for _, img := range result.OnlyIn1 {
		fmt.Printf("  [DEL]  %s (only in first document)\n", img.Name)
	}
	for _, img := range result.OnlyIn2 {
		fmt.Printf("  [ADD]  %s (only in second document)\n", img.Name)
	}

	if len(result.Skipped) > 0 && verbose {
		for _, img := range result.Skipped {
			fmt.Printf("  [SKIP] %s\n", img.Name)
		}
	}

	total := len(result.Different) + len(result.OnlyIn1) + len(result.OnlyIn2)
	if total == 0 {
		fmt.Println("  No image differences found.")
	} else {
		fmt.Printf("  %d difference(s) found.\n", total)
	}
}
