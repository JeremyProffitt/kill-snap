package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/rwcarlsen/goexif/exif"
)

type ImageMetadata struct {
	ImageGUID        string            `json:"ImageGUID"`
	OriginalFile     string            `json:"OriginalFile"`
	Bucket           string            `json:"Bucket"`
	Thumbnail50      string            `json:"Thumbnail50"`
	Thumbnail400     string            `json:"Thumbnail400"`
	RelatedFiles     []string          `json:"RelatedFiles"`
	EXIFData         map[string]string `json:"EXIFData"`
	Width            int               `json:"Width"`
	Height           int               `json:"Height"`
	FileSize         int64             `json:"FileSize"`
	Reviewed         string            `json:"Reviewed"`
	InsertedDateTime string            `json:"InsertedDateTime"`
	UpdatedDateTime  string            `json:"UpdatedDateTime"`
}

var (
	s3Client  *s3.S3
	ddbClient *dynamodb.DynamoDB
	tableName string
)

func init() {
	sess := session.Must(session.NewSession())
	s3Client = s3.New(sess)
	ddbClient = dynamodb.New(sess)
	tableName = os.Getenv("DYNAMODB_TABLE")
}

func handler(ctx context.Context, s3Event events.S3Event) error {
	for _, record := range s3Event.Records {
		bucket := record.S3.Bucket.Name
		key := record.S3.Object.Key

		// Skip if this is already a thumbnail
		if strings.Contains(key, ".50.") || strings.Contains(key, ".400.") {
			continue
		}

		fmt.Printf("Processing file: %s from bucket: %s\n", key, bucket)

		// Download the original image
		result, err := s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return fmt.Errorf("failed to get object: %v", err)
		}
		defer result.Body.Close()

		// Decode the image
		img, format, err := image.Decode(result.Body)
		if err != nil {
			return fmt.Errorf("failed to decode image: %v", err)
		}

		// Get original dimensions
		bounds := img.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()

		// Extract EXIF data
		result.Body.Close() // Close previous read
		result, _ = s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		// Read body into buffer for EXIF extraction
		exifBuf := new(bytes.Buffer)
		exifBuf.ReadFrom(result.Body)
		exifData := extractEXIF(bytes.NewReader(exifBuf.Bytes()))
		result.Body.Close()

		// Generate thumbnails (150px for gallery grid, 400px for modal preview)
		thumbnail50, err := generateThumbnail(img, 150)
		if err != nil {
			return fmt.Errorf("failed to generate 150px thumbnail: %v", err)
		}

		thumbnail400, err := generateThumbnail(img, 400)
		if err != nil {
			return fmt.Errorf("failed to generate 400px thumbnail: %v", err)
		}

		// Generate thumbnail file names
		ext := filepath.Ext(key)
		baseName := strings.TrimSuffix(key, ext)
		thumbnail50Key := baseName + ".50" + ext
		thumbnail400Key := baseName + ".400" + ext

		// Upload thumbnails to S3
		if err := uploadThumbnail(bucket, thumbnail50Key, thumbnail50); err != nil {
			return fmt.Errorf("failed to upload 50px thumbnail: %v", err)
		}

		if err := uploadThumbnail(bucket, thumbnail400Key, thumbnail400); err != nil {
			return fmt.Errorf("failed to upload 400px thumbnail: %v", err)
		}

		// Find related files with same base name
		relatedFiles, err := findRelatedFiles(bucket, baseName)
		if err != nil {
			fmt.Printf("Warning: failed to find related files: %v\n", err)
		}

		// Create metadata record
		now := time.Now().Format(time.RFC3339)
		metadata := ImageMetadata{
			ImageGUID:        uuid.New().String(),
			OriginalFile:     key,
			Bucket:           bucket,
			Thumbnail50:      thumbnail50Key,
			Thumbnail400:     thumbnail400Key,
			RelatedFiles:     relatedFiles,
			EXIFData:         exifData,
			Width:            width,
			Height:           height,
			FileSize:         *result.ContentLength,
			Reviewed:         "false",
			InsertedDateTime: now,
			UpdatedDateTime:  now,
		}

		// Store in DynamoDB
		if err := storeMetadata(metadata); err != nil {
			return fmt.Errorf("failed to store metadata: %v", err)
		}

		fmt.Printf("Successfully processed %s - GUID: %s\n", key, metadata.ImageGUID)
		fmt.Printf("Format: %s\n", format)
	}

	return nil
}

func generateThumbnail(img image.Image, height int) (*bytes.Buffer, error) {
	// Resize maintaining aspect ratio using high-quality Lanczos resampling
	thumbnail := imaging.Resize(img, 0, height, imaging.Lanczos)

	// Apply mild sharpening to improve clarity after resize
	thumbnail = imaging.Sharpen(thumbnail, 0.5)

	// Encode to JPEG with high quality
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, thumbnail, &jpeg.Options{Quality: 92}); err != nil {
		return nil, err
	}

	return buf, nil
}

func uploadThumbnail(bucket, key string, buf *bytes.Buffer) error {
	_, err := s3Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String("image/jpeg"),
	})
	return err
}

func extractEXIF(body *bytes.Reader) map[string]string {
	exifData := make(map[string]string)

	x, err := exif.Decode(body)
	if err != nil {
		fmt.Printf("Warning: failed to decode EXIF: %v\n", err)
		return exifData
	}

	// Extract common EXIF fields
	fields := []exif.FieldName{
		exif.Make,
		exif.Model,
		exif.DateTime,
		exif.DateTimeOriginal,
		exif.DateTimeDigitized,
		exif.Orientation,
		exif.XResolution,
		exif.YResolution,
		exif.Software,
		exif.Artist,
		exif.Copyright,
	}

	for _, field := range fields {
		tag, err := x.Get(field)
		if err == nil {
			exifData[string(field)] = tag.String()
		}
	}

	return exifData
}

func findRelatedFiles(bucket, baseName string) ([]string, error) {
	var relatedFiles []string

	// List objects with the same base name
	result, err := s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(baseName),
	})
	if err != nil {
		return nil, err
	}

	for _, obj := range result.Contents {
		key := *obj.Key
		// Skip the original file and thumbnails
		if !strings.HasSuffix(key, ".jpg") || strings.Contains(key, ".50.") || strings.Contains(key, ".400.") {
			if key != baseName+".jpg" && key != baseName+".JPG" {
				relatedFiles = append(relatedFiles, key)
			}
		}
	}

	return relatedFiles, nil
}

func storeMetadata(metadata ImageMetadata) error {
	av, err := dynamodbattribute.MarshalMap(metadata)
	if err != nil {
		return err
	}

	_, err = ddbClient.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	})
	return err
}

func main() {
	lambda.Start(handler)
}
