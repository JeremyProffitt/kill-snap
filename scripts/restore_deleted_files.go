// restore_deleted_files.go - Restores deleted files from S3 versioning
//
// This script scans S3 project folders for delete markers and removes them
// to restore the original files.
//
// Usage:
//   go run restore_deleted_files.go                    # Dry run - show what would be restored
//   go run restore_deleted_files.go -apply             # Apply the restorations
//   go run restore_deleted_files.go -apply -verbose    # Apply with detailed output

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	defaultBucket = "kill-snap"
	defaultRegion = "us-east-2"
	projectPrefix = "projects/"
)

var (
	bucketName string
	awsRegion  string
)

func init() {
	bucketName = getEnvOrDefault("S3_BUCKET", defaultBucket)
	awsRegion = getEnvOrDefault("AWS_REGION", defaultRegion)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// DeleteMarkerInfo holds information about a delete marker
type DeleteMarkerInfo struct {
	Key       string
	VersionID string
	Project   string
	FileName  string
	FileType  string // "original", "thumbnail", "raw"
}

// Stats tracks restoration statistics
type Stats struct {
	TotalDeleteMarkers int
	Originals          int
	Thumbnails         int
	RawFiles           int
	Restored           int
	Errors             int
	ByProject          map[string]int
}

func main() {
	apply := flag.Bool("apply", false, "Apply the restorations (default is dry-run)")
	verbose := flag.Bool("verbose", false, "Show detailed output")
	flag.Parse()

	fmt.Printf("Bucket: %s\n", bucketName)
	fmt.Printf("Region: %s\n", awsRegion)
	fmt.Printf("Mode: %s\n", map[bool]string{true: "APPLY", false: "DRY-RUN"}[*apply])
	fmt.Println()

	// Create AWS session
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))
	s3Client := s3.New(sess)

	stats := Stats{
		ByProject: make(map[string]int),
	}

	// List all delete markers in project folders
	fmt.Println("Scanning for delete markers in projects/...")
	deleteMarkers := []DeleteMarkerInfo{}

	var keyMarker, versionMarker *string
	for {
		input := &s3.ListObjectVersionsInput{
			Bucket: aws.String(bucketName),
			Prefix: aws.String(projectPrefix),
		}
		if keyMarker != nil {
			input.KeyMarker = keyMarker
		}
		if versionMarker != nil {
			input.VersionIdMarker = versionMarker
		}

		result, err := s3Client.ListObjectVersions(input)
		if err != nil {
			fmt.Printf("Error listing object versions: %v\n", err)
			os.Exit(1)
		}

		// Process delete markers
		for _, dm := range result.DeleteMarkers {
			if dm.IsLatest == nil || !*dm.IsLatest {
				continue // Only process current delete markers
			}

			key := *dm.Key
			versionID := *dm.VersionId

			// Parse project name and file info
			parts := strings.SplitN(strings.TrimPrefix(key, projectPrefix), "/", 2)
			project := ""
			if len(parts) > 0 {
				project = parts[0]
			}

			fileName := key[strings.LastIndex(key, "/")+1:]
			fileType := classifyFile(fileName)

			marker := DeleteMarkerInfo{
				Key:       key,
				VersionID: versionID,
				Project:   project,
				FileName:  fileName,
				FileType:  fileType,
			}
			deleteMarkers = append(deleteMarkers, marker)

			stats.TotalDeleteMarkers++
			stats.ByProject[project]++

			switch fileType {
			case "original":
				stats.Originals++
			case "thumbnail":
				stats.Thumbnails++
			case "raw":
				stats.RawFiles++
			}

			if *verbose {
				fmt.Printf("  Found: %s [%s] in %s\n", fileName, fileType, project)
			}
		}

		// Check for more pages
		if result.IsTruncated == nil || !*result.IsTruncated {
			break
		}
		keyMarker = result.NextKeyMarker
		versionMarker = result.NextVersionIdMarker
	}

	fmt.Printf("\nFound %d delete markers\n", stats.TotalDeleteMarkers)
	fmt.Printf("  Originals:  %d\n", stats.Originals)
	fmt.Printf("  Thumbnails: %d\n", stats.Thumbnails)
	fmt.Printf("  RAW files:  %d\n", stats.RawFiles)
	fmt.Println()

	fmt.Println("By project:")
	for project, count := range stats.ByProject {
		fmt.Printf("  %s: %d files\n", project, count)
	}
	fmt.Println()

	if !*apply {
		fmt.Println("This was a DRY RUN. To restore these files, run:")
		fmt.Println("  go run restore_deleted_files.go -apply")
		return
	}

	// Restore files by removing delete markers
	fmt.Println("Restoring files...")
	for _, marker := range deleteMarkers {
		if *verbose {
			fmt.Printf("  Restoring: %s\n", marker.Key)
		}

		_, err := s3Client.DeleteObject(&s3.DeleteObjectInput{
			Bucket:    aws.String(bucketName),
			Key:       aws.String(marker.Key),
			VersionId: aws.String(marker.VersionID),
		})

		if err != nil {
			fmt.Printf("  ERROR restoring %s: %v\n", marker.Key, err)
			stats.Errors++
		} else {
			stats.Restored++
		}
	}

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Total delete markers found: %d\n", stats.TotalDeleteMarkers)
	fmt.Printf("Successfully restored:      %d\n", stats.Restored)
	if stats.Errors > 0 {
		fmt.Printf("Errors:                     %d\n", stats.Errors)
	}
}

// classifyFile determines the type of file based on its name
func classifyFile(fileName string) string {
	lower := strings.ToLower(fileName)

	// Check for thumbnails
	if strings.Contains(lower, ".50.") || strings.Contains(lower, ".400.") {
		return "thumbnail"
	}

	// Check for RAW files
	rawExtensions := []string{".raf", ".cr2", ".cr3", ".nef", ".arw", ".dng", ".orf", ".rw2", ".pef", ".srw"}
	for _, ext := range rawExtensions {
		if strings.HasSuffix(lower, ext) {
			return "raw"
		}
	}

	// Check for images
	if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".png") {
		return "original"
	}

	return "other"
}
