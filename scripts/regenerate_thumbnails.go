// regenerate_thumbnails.go - Regenerates missing thumbnails for project images
//
// This script downloads original images, creates thumbnails, uploads them to S3,
// and updates DynamoDB records.
//
// Usage:
//   go run regenerate_thumbnails.go                    # Dry run
//   go run regenerate_thumbnails.go -apply             # Apply fixes

package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/jpeg"
	"io"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/disintegration/imaging"
)

const (
	defaultBucket     = "kill-snap"
	defaultImageTable = "kill-snap-ImageMetadata"
	awsRegion         = "us-east-2"
)

// ImageRecord represents a DynamoDB image record
type ImageRecord struct {
	ImageGUID    string `dynamodbav:"ImageGUID"`
	OriginalFile string `dynamodbav:"OriginalFile"`
	Thumbnail50  string `dynamodbav:"Thumbnail50"`
	Thumbnail400 string `dynamodbav:"Thumbnail400"`
	Status       string `dynamodbav:"Status"`
}

func main() {
	apply := flag.Bool("apply", false, "Apply the fixes (default is dry-run)")
	flag.Parse()

	bucketName := getEnvOrDefault("S3_BUCKET", defaultBucket)
	imageTable := getEnvOrDefault("IMAGE_TABLE", defaultImageTable)

	fmt.Printf("Image Table: %s\n", imageTable)
	fmt.Printf("S3 Bucket: %s\n", bucketName)
	fmt.Printf("Mode: %s\n", map[bool]string{true: "APPLY", false: "DRY-RUN"}[*apply])
	fmt.Println()

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))
	ddbClient := dynamodb.New(sess)
	s3Client := s3.New(sess)

	// Scan for project images with missing thumbnails
	fmt.Println("Scanning for project images needing thumbnail regeneration...")

	var needsRegen []ImageRecord
	var lastKey map[string]*dynamodb.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName:        aws.String(imageTable),
			FilterExpression: aws.String("#status = :project AND begins_with(OriginalFile, :prefix)"),
			ExpressionAttributeNames: map[string]*string{
				"#status": aws.String("Status"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":project": {S: aws.String("project")},
				":prefix":  {S: aws.String("projects/")},
			},
		}
		if lastKey != nil {
			input.ExclusiveStartKey = lastKey
		}

		result, err := ddbClient.Scan(input)
		if err != nil {
			fmt.Printf("Error scanning: %v\n", err)
			os.Exit(1)
		}

		for _, item := range result.Items {
			var record ImageRecord
			if err := dynamodbattribute.UnmarshalMap(item, &record); err != nil {
				continue
			}

			// Check if thumbnail paths are missing
			if record.Thumbnail50 == "" || record.Thumbnail400 == "" {
				// Check if thumbnails exist in S3
				dir := path.Dir(record.OriginalFile)
				baseName := strings.TrimSuffix(path.Base(record.OriginalFile), path.Ext(record.OriginalFile))
				ext := strings.ToLower(path.Ext(record.OriginalFile))

				thumb50 := fmt.Sprintf("%s/%s.50%s", dir, baseName, ext)
				thumb400 := fmt.Sprintf("%s/%s.400%s", dir, baseName, ext)

				thumb50Exists := checkS3Exists(s3Client, bucketName, thumb50)
				thumb400Exists := checkS3Exists(s3Client, bucketName, thumb400)

				if !thumb50Exists || !thumb400Exists {
					needsRegen = append(needsRegen, record)
				}
			}
		}

		lastKey = result.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}

	fmt.Printf("Found %d records needing thumbnail regeneration\n\n", len(needsRegen))

	if len(needsRegen) == 0 {
		fmt.Println("No thumbnails need regeneration!")
		return
	}

	// Regenerate thumbnails
	regenerated := 0
	errors := 0

	for _, record := range needsRegen {
		dir := path.Dir(record.OriginalFile)
		baseName := strings.TrimSuffix(path.Base(record.OriginalFile), path.Ext(record.OriginalFile))
		ext := strings.ToLower(path.Ext(record.OriginalFile))

		thumb50Key := fmt.Sprintf("%s/%s.50%s", dir, baseName, ext)
		thumb400Key := fmt.Sprintf("%s/%s.400%s", dir, baseName, ext)

		fmt.Printf("  %s: %s\n", record.ImageGUID, record.OriginalFile)

		if !*apply {
			fmt.Printf("    Would generate: %s, %s\n", thumb50Key, thumb400Key)
			regenerated++
			continue
		}

		// Download original image
		fmt.Printf("    Downloading original...\n")
		getResult, err := s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(record.OriginalFile),
		})
		if err != nil {
			fmt.Printf("    ERROR downloading: %v\n", err)
			errors++
			continue
		}

		imgData, err := io.ReadAll(getResult.Body)
		getResult.Body.Close()
		if err != nil {
			fmt.Printf("    ERROR reading: %v\n", err)
			errors++
			continue
		}

		// Decode image
		img, err := imaging.Decode(bytes.NewReader(imgData))
		if err != nil {
			fmt.Printf("    ERROR decoding: %v\n", err)
			errors++
			continue
		}

		// Generate 50px thumbnail
		thumb50 := imaging.Fit(img, 50, 50, imaging.Lanczos)
		var thumb50Buf bytes.Buffer
		if err := jpeg.Encode(&thumb50Buf, thumb50, &jpeg.Options{Quality: 80}); err != nil {
			fmt.Printf("    ERROR encoding 50px: %v\n", err)
			errors++
			continue
		}

		// Generate 400px thumbnail
		thumb400 := imaging.Fit(img, 400, 400, imaging.Lanczos)
		var thumb400Buf bytes.Buffer
		if err := jpeg.Encode(&thumb400Buf, thumb400, &jpeg.Options{Quality: 85}); err != nil {
			fmt.Printf("    ERROR encoding 400px: %v\n", err)
			errors++
			continue
		}

		// Upload thumbnails
		fmt.Printf("    Uploading thumbnails...\n")
		_, err = s3Client.PutObject(&s3.PutObjectInput{
			Bucket:      aws.String(bucketName),
			Key:         aws.String(thumb50Key),
			Body:        bytes.NewReader(thumb50Buf.Bytes()),
			ContentType: aws.String("image/jpeg"),
		})
		if err != nil {
			fmt.Printf("    ERROR uploading 50px: %v\n", err)
			errors++
			continue
		}

		_, err = s3Client.PutObject(&s3.PutObjectInput{
			Bucket:      aws.String(bucketName),
			Key:         aws.String(thumb400Key),
			Body:        bytes.NewReader(thumb400Buf.Bytes()),
			ContentType: aws.String("image/jpeg"),
		})
		if err != nil {
			fmt.Printf("    ERROR uploading 400px: %v\n", err)
			errors++
			continue
		}

		// Update DynamoDB record
		fmt.Printf("    Updating DynamoDB...\n")
		_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(record.ImageGUID)},
			},
			UpdateExpression: aws.String("SET Thumbnail50 = :t50, Thumbnail400 = :t400"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":t50":  {S: aws.String(thumb50Key)},
				":t400": {S: aws.String(thumb400Key)},
			},
		})
		if err != nil {
			fmt.Printf("    ERROR updating DynamoDB: %v\n", err)
			errors++
			continue
		}

		fmt.Printf("    Done!\n")
		regenerated++
	}

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Records needing regeneration: %d\n", len(needsRegen))
	if *apply {
		fmt.Printf("Successfully regenerated:     %d\n", regenerated)
		if errors > 0 {
			fmt.Printf("Errors:                       %d\n", errors)
		}
	} else {
		fmt.Printf("Would regenerate:             %d\n", regenerated)
		fmt.Println("\nThis was a DRY RUN. To regenerate thumbnails, run:")
		fmt.Println("  go run regenerate_thumbnails.go -apply")
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func checkS3Exists(s3Client *s3.S3, bucket, key string) bool {
	_, err := s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err == nil
}
