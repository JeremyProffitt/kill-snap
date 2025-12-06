package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
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
	Keywords         []string          `json:"Keywords,omitempty"`
	Description      string            `json:"Description,omitempty"`
	InsertedDateTime string            `json:"InsertedDateTime"`
	UpdatedDateTime  string            `json:"UpdatedDateTime"`
}

// OpenAI API types
type OpenAIMessage struct {
	Role    string        `json:"role"`
	Content []interface{} `json:"content"`
}

type OpenAITextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type OpenAIImageContent struct {
	Type     string         `json:"type"`
	ImageURL OpenAIImageURL `json:"image_url"`
}

type OpenAIImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail"`
}

type OpenAIRequest struct {
	Model     string          `json:"model"`
	Messages  []OpenAIMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type AIAnalysisResult struct {
	Keywords    []string `json:"keywords"`
	Description string   `json:"description"`
}

var (
	s3Client     *s3.S3
	ddbClient    *dynamodb.DynamoDB
	tableName    string
	openaiAPIKey string
)

func init() {
	sess := session.Must(session.NewSession())
	s3Client = s3.New(sess)
	ddbClient = dynamodb.New(sess)
	tableName = os.Getenv("DYNAMODB_TABLE")
	openaiAPIKey = os.Getenv("OPENAI_API_KEY")
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

		// Generate thumbnails (150px for small icons, 800px for gallery/modal - higher quality)
		thumbnail50, err := generateThumbnail(img, 150)
		if err != nil {
			return fmt.Errorf("failed to generate 150px thumbnail: %v", err)
		}

		thumbnail400, err := generateThumbnail(img, 800)
		if err != nil {
			return fmt.Errorf("failed to generate 800px thumbnail: %v", err)
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

		// Generate AI keywords and description if OpenAI is configured
		if openaiAPIKey != "" {
			aiResult, err := analyzeImageWithAI(thumbnail400)
			if err != nil {
				fmt.Printf("Warning: AI analysis failed: %v\n", err)
			} else {
				metadata.Keywords = aiResult.Keywords
				metadata.Description = aiResult.Description
				fmt.Printf("AI analysis complete: %d keywords\n", len(aiResult.Keywords))
			}
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

func analyzeImageWithAI(thumbnailData *bytes.Buffer) (*AIAnalysisResult, error) {
	// Base64 encode the thumbnail
	imageBase64 := base64.StdEncoding.EncodeToString(thumbnailData.Bytes())
	dataURL := fmt.Sprintf("data:image/jpeg;base64,%s", imageBase64)

	// Build OpenAI request
	prompt := `Analyze this image and provide:
1. A list of relevant keywords (5-15 keywords) that describe the subject, setting, mood, colors, and composition
2. A brief description (1-2 sentences) of what the image shows

Return your response as JSON in this exact format:
{"keywords": ["keyword1", "keyword2", ...], "description": "Your description here"}`

	openaiReq := OpenAIRequest{
		Model: "gpt-4o",
		Messages: []OpenAIMessage{
			{
				Role: "user",
				Content: []interface{}{
					OpenAITextContent{Type: "text", Text: prompt},
					OpenAIImageContent{
						Type: "image_url",
						ImageURL: OpenAIImageURL{
							URL:    dataURL,
							Detail: "low",
						},
					},
				},
			},
		},
		MaxTokens: 500,
	}

	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Make HTTP request to OpenAI
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+openaiAPIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var openaiResp OpenAIResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if openaiResp.Error != nil {
		return nil, fmt.Errorf("OpenAI error: %s", openaiResp.Error.Message)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	// Parse the JSON response from the model
	content := openaiResp.Choices[0].Message.Content

	// Try to extract JSON from the response (it might have markdown code blocks)
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var result AIAnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse AI response JSON: %v (content: %s)", err, content)
	}

	return &result, nil
}

func main() {
	lambda.Start(handler)
}
