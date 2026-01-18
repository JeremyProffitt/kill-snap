package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	bucket = "kill-snap"
	region = "us-east-2"
)

var (
	s3Client *s3.S3
)

func init() {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(region),
	}))
	s3Client = s3.New(sess)
}

// extractJPEGFromRAF extracts the embedded JPEG preview from a Fujifilm RAF file
func extractJPEGFromRAF(rafData []byte) ([]byte, error) {
	// RAF file structure (Fujifilm):
	// Offset 0: "FUJIFILMCCD-RAW " (16 bytes)
	// Offset 16: Format version (4 bytes)
	// Offset 20: Camera ID (8 bytes)
	// Offset 28: RAF version string (4 bytes)
	// Offset 84: JPEG image offset (4 bytes, big-endian)
	// Offset 88: JPEG image length (4 bytes, big-endian)

	if len(rafData) < 92 {
		return nil, fmt.Errorf("file too small to be valid RAF")
	}

	// Check header
	if string(rafData[:15]) != "FUJIFILMCCD-RAW" {
		return nil, fmt.Errorf("not a valid RAF file (header mismatch)")
	}

	// Read JPEG offset and length from header
	jpegOffset := binary.BigEndian.Uint32(rafData[84:88])
	jpegLength := binary.BigEndian.Uint32(rafData[88:92])

	fmt.Printf("  RAF header: JPEG offset=%d, length=%d\n", jpegOffset, jpegLength)

	// Validate offset and length
	if jpegOffset == 0 || jpegLength == 0 {
		fmt.Printf("  Header values invalid, falling back to scan method\n")
		return extractJPEGByScan(rafData)
	}

	if int(jpegOffset+jpegLength) > len(rafData) {
		fmt.Printf("  Header values out of bounds, falling back to scan method\n")
		return extractJPEGByScan(rafData)
	}

	// Extract JPEG from specified location
	jpegData := rafData[jpegOffset : jpegOffset+jpegLength]

	// Verify it's actually a JPEG
	if len(jpegData) < 3 || jpegData[0] != 0xFF || jpegData[1] != 0xD8 || jpegData[2] != 0xFF {
		fmt.Printf("  Data at offset is not JPEG, falling back to scan method\n")
		return extractJPEGByScan(rafData)
	}

	fmt.Printf("  Extracted JPEG from header location: %d bytes\n", len(jpegData))
	return jpegData, nil
}

// extractJPEGByScan finds the largest embedded JPEG by scanning the file
func extractJPEGByScan(rafData []byte) ([]byte, error) {
	var allJPEGs [][]byte
	searchStart := 0

	// Find all embedded JPEGs
	for searchStart < len(rafData)-3 {
		// Find JPEG start marker (FF D8 FF)
		jpegStart := -1
		for i := searchStart; i < len(rafData)-3; i++ {
			if rafData[i] == 0xFF && rafData[i+1] == 0xD8 && rafData[i+2] == 0xFF {
				jpegStart = i
				break
			}
		}

		if jpegStart == -1 {
			break
		}

		// Find JPEG end marker (FF D9)
		jpegEnd := -1
		for i := jpegStart + 3; i < len(rafData)-1; i++ {
			if rafData[i] == 0xFF && rafData[i+1] == 0xD9 {
				jpegEnd = i + 2
				break
			}
		}

		if jpegEnd == -1 {
			searchStart = jpegStart + 1
			continue
		}

		jpegData := rafData[jpegStart:jpegEnd]
		allJPEGs = append(allJPEGs, jpegData)
		fmt.Printf("  Found JPEG by scan: %d bytes (offset %d to %d)\n", len(jpegData), jpegStart, jpegEnd)

		searchStart = jpegEnd
	}

	if len(allJPEGs) == 0 {
		return nil, fmt.Errorf("no JPEG found in RAF file")
	}

	// Return the largest JPEG
	var largest []byte
	for _, jpeg := range allJPEGs {
		if len(jpeg) > len(largest) {
			largest = jpeg
		}
	}

	fmt.Printf("  Selected largest JPEG: %d bytes\n", len(largest))
	return largest, nil
}

// extractJPEGFromGenericRAW attempts to extract embedded JPEG from various RAW formats
func extractJPEGFromGenericRAW(rawData []byte) ([]byte, error) {
	// Most RAW formats embed a JPEG preview
	// Look for the largest JPEG segment in the file

	var bestJPEG []byte
	searchStart := 0

	for searchStart < len(rawData)-3 {
		// Find next JPEG start
		jpegStart := -1
		for i := searchStart; i < len(rawData)-3; i++ {
			if rawData[i] == 0xFF && rawData[i+1] == 0xD8 && rawData[i+2] == 0xFF {
				jpegStart = i
				break
			}
		}

		if jpegStart == -1 {
			break
		}

		// Find corresponding end
		jpegEnd := -1
		for i := jpegStart + 3; i < len(rawData)-1; i++ {
			if rawData[i] == 0xFF && rawData[i+1] == 0xD9 {
				jpegEnd = i + 2
				break
			}
		}

		if jpegEnd == -1 {
			searchStart = jpegStart + 1
			continue
		}

		jpegData := rawData[jpegStart:jpegEnd]

		// Keep the largest JPEG found (usually the full preview)
		if len(jpegData) > len(bestJPEG) {
			bestJPEG = jpegData
		}

		searchStart = jpegEnd
	}

	if len(bestJPEG) == 0 {
		return nil, fmt.Errorf("no embedded JPEG found in RAW file")
	}

	// Sanity check - embedded preview should be at least 100KB
	if len(bestJPEG) < 100*1024 {
		fmt.Printf("  Warning: Extracted JPEG is small (%d bytes), may be thumbnail only\n", len(bestJPEG))
	}

	return bestJPEG, nil
}

func processRAFFile(key string, dryRun bool) error {
	// Use path (not filepath) to handle S3 keys correctly on Windows
	baseName := strings.TrimSuffix(path.Base(key), path.Ext(key))
	jpgKey := path.Dir(key) + "/" + baseName + ".JPG"

	fmt.Printf("Processing: %s\n", key)

	// Check if JPG already exists
	_, err := s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(jpgKey),
	})
	if err == nil {
		fmt.Printf("  JPG already exists: %s (skipping)\n", jpgKey)
		return nil
	}

	// Download RAF file
	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to download %s: %v", key, err)
	}
	defer result.Body.Close()

	// Check file size
	if result.ContentLength != nil && *result.ContentLength == 0 {
		fmt.Printf("  ERROR: File is empty (0 bytes), skipping\n")
		return nil
	}

	rafData, err := io.ReadAll(result.Body)
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", key, err)
	}

	fmt.Printf("  Downloaded: %d bytes\n", len(rafData))

	if len(rafData) < 1000 {
		fmt.Printf("  ERROR: File too small (%d bytes), likely corrupted\n", len(rafData))
		return nil
	}

	// Check if it's a valid RAF file
	if len(rafData) > 15 && string(rafData[:15]) == "FUJIFILMCCD-RAW" {
		fmt.Printf("  Valid Fujifilm RAF file detected\n")
	}

	// Extract embedded JPEG
	jpegData, err := extractJPEGFromRAF(rafData)
	if err != nil {
		// Try generic extraction
		jpegData, err = extractJPEGFromGenericRAW(rafData)
		if err != nil {
			return fmt.Errorf("failed to extract JPEG from %s: %v", key, err)
		}
	}

	if dryRun {
		fmt.Printf("  DRY RUN: Would upload %s (%d bytes)\n", jpgKey, len(jpegData))
		return nil
	}

	// Upload extracted JPEG
	fmt.Printf("  Uploading to bucket=%s key=%s size=%d\n", bucket, jpgKey, len(jpegData))
	putOutput, err := s3Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(jpgKey),
		Body:        bytes.NewReader(jpegData),
		ContentType: aws.String("image/jpeg"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload %s: %v", jpgKey, err)
	}

	fmt.Printf("  Uploaded: %s (%d bytes) ETag: %s\n", jpgKey, len(jpegData), *putOutput.ETag)
	return nil
}

func listRAFFiles() ([]string, error) {
	var rafFiles []string

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String("incoming/"),
	}

	err := s3Client.ListObjectsV2Pages(input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			key := *obj.Key
			ext := strings.ToLower(path.Ext(key))
			// Only include RAF files over 20MB (smaller ones are likely truncated/corrupted)
			if ext == ".raf" && *obj.Size > 20*1024*1024 {
				rafFiles = append(rafFiles, key)
			}
		}
		return true
	})

	return rafFiles, err
}

func main() {
	dryRun := flag.Bool("dry-run", false, "Show what would be done without making changes")
	limit := flag.Int("limit", 10, "Maximum number of files to process")
	single := flag.String("file", "", "Process a single file by key")
	flag.Parse()

	fmt.Println("=== RAF Preview Extractor ===")
	fmt.Printf("Bucket: %s\n", bucket)
	fmt.Printf("Region: %s\n\n", region)

	if *single != "" {
		// Process single file
		if err := processRAFFile(*single, *dryRun); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// List and process RAF files
	fmt.Println("Scanning for RAF files in incoming/...")
	rafFiles, err := listRAFFiles()
	if err != nil {
		fmt.Printf("Error listing files: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d RAF files with content\n\n", len(rafFiles))

	if len(rafFiles) == 0 {
		fmt.Println("No RAF files to process")
		return
	}

	processed := 0
	errors := 0

	for _, key := range rafFiles {
		if processed >= *limit {
			fmt.Printf("\nReached limit of %d files\n", *limit)
			break
		}

		if err := processRAFFile(key, *dryRun); err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			errors++
		}
		processed++
		fmt.Println()
	}

	fmt.Printf("\nSummary: Processed %d files, %d errors\n", processed, errors)
	if *dryRun {
		fmt.Println("(DRY RUN - no changes made)")
	}
}

// Helper to read uint32 big endian
func readUint32BE(data []byte) uint32 {
	return binary.BigEndian.Uint32(data)
}
