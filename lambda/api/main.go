package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	lrcat "github.com/JeremyProffitt/lrcat-go"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	lambdasvc "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	sess              *session.Session
	ddbClient         *dynamodb.DynamoDB
	s3Client          *s3.S3
	lambdaClient      *lambdasvc.Lambda
	cwLogsClient      *cloudwatchlogs.CloudWatchLogs
	bucketName        string
	imageTable        string
	usersTable        string
	reviewGroupsTable string
	projectsTable     string
	adminUsername     string
	adminPassword     string
	functionName      string
	openaiAPIKey      string
	zipLambdaName     string
	jwtSecret         = []byte("kill-snap-secret-key-change-in-production")
)

func init() {
	sess = session.Must(session.NewSession())
	ddbClient = dynamodb.New(sess)
	s3Client = s3.New(sess)
	lambdaClient = lambdasvc.New(sess)
	cwLogsClient = cloudwatchlogs.New(sess)
	bucketName = os.Getenv("BUCKET_NAME")
	imageTable = os.Getenv("IMAGE_TABLE")
	usersTable = os.Getenv("USERS_TABLE")
	reviewGroupsTable = os.Getenv("REVIEW_GROUPS_TABLE")
	projectsTable = os.Getenv("PROJECTS_TABLE")
	adminUsername = os.Getenv("ADMIN_USERNAME")
	adminPassword = os.Getenv("ADMIN_PASSWORD")
	functionName = os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
	openaiAPIKey = os.Getenv("OPENAI_API_KEY")
	zipLambdaName = os.Getenv("ZIP_LAMBDA_NAME")

	// Initialize admin user if it doesn't exist
	if adminUsername != "" && adminPassword != "" {
		initializeAdminUser()
	}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type ImageResponse struct {
	ImageGUID        string            `json:"imageGUID"`
	OriginalFile     string            `json:"originalFile"`
	Thumbnail50      string            `json:"thumbnail50"`
	Thumbnail400     string            `json:"thumbnail400"`
	Bucket           string            `json:"bucket"`
	Width            int               `json:"width"`
	Height           int               `json:"height"`
	FileSize         int64             `json:"fileSize"`
	Reviewed         string            `json:"reviewed"`
	GroupNumber      int               `json:"groupNumber,omitempty"`
	ColorCode        string            `json:"colorCode,omitempty"`
	Rating           int               `json:"rating,omitempty"`
	Promoted         bool              `json:"promoted,omitempty"`
	Keywords         []string          `json:"keywords,omitempty"`
	Description      string            `json:"description,omitempty"` // AI-generated description
	EXIFData         map[string]string `json:"exifData,omitempty"`
	RelatedFiles     []string          `json:"relatedFiles,omitempty"`
	InsertedDateTime string            `json:"insertedDateTime,omitempty"`
	UpdatedDateTime  string            `json:"updatedDateTime,omitempty"`
	MoveStatus       string            `json:"moveStatus,omitempty"` // "pending", "moving", "complete", "failed"
	Status           string            `json:"status,omitempty"`     // "inbox", "approved", "rejected", "deleted", "project"
	ProjectID        string            `json:"projectId,omitempty"`
}

type UpdateImageRequest struct {
	GroupNumber int      `json:"groupNumber,omitempty"`
	ColorCode   string   `json:"colorCode,omitempty"`
	Rating      int      `json:"rating,omitempty"`
	Promoted    bool     `json:"promoted,omitempty"`
	Reviewed    string   `json:"reviewed,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

type Project struct {
	ProjectID        string    `json:"projectId" dynamodbav:"ProjectID"`
	Name             string    `json:"name" dynamodbav:"Name"`
	S3Prefix         string    `json:"s3Prefix,omitempty" dynamodbav:"S3Prefix,omitempty"`
	CreatedAt        string    `json:"createdAt" dynamodbav:"CreatedAt"`
	ImageCount       int       `json:"imageCount" dynamodbav:"ImageCount"`
	CatalogPath      string    `json:"catalogPath,omitempty" dynamodbav:"CatalogPath,omitempty"`
	CatalogUpdatedAt string    `json:"catalogUpdatedAt,omitempty" dynamodbav:"CatalogUpdatedAt,omitempty"`
	Keywords         []string  `json:"keywords,omitempty" dynamodbav:"Keywords,omitempty"`
	ZipFiles         []ZipFile `json:"zipFiles,omitempty" dynamodbav:"ZipFiles,omitempty"`
}

// ZipFile represents a generated zip file for a project
type ZipFile struct {
	Key        string `json:"key" dynamodbav:"Key"`
	Size       int64  `json:"size" dynamodbav:"Size"`
	ImageCount int    `json:"imageCount" dynamodbav:"ImageCount"`
	CreatedAt  string `json:"createdAt" dynamodbav:"CreatedAt"`
	Status     string `json:"status" dynamodbav:"Status"` // "generating", "complete", "failed"
}

type CreateProjectRequest struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords,omitempty"`
}

type UpdateProjectRequest struct {
	Name     string   `json:"name,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
}

type AddToProjectRequest struct {
	All   bool `json:"all,omitempty"`
	Group int  `json:"group,omitempty"`
}

// AsyncMoveRequest is used for async Lambda invocation to move files
type AsyncMoveRequest struct {
	Action      string `json:"action"` // "move_files"
	ImageGUID   string `json:"imageGUID"`
	DestPrefix  string `json:"destPrefix"`
	NewStatus   string `json:"newStatus"` // "approved", "rejected", "deleted"
	Bucket      string `json:"bucket"`
}

// OpenAI API types for GPT-4o vision analysis
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
	Detail string `json:"detail"` // "high" for high detail
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

func initializeAdminUser() {
	fmt.Printf("Initializing admin user: %s\n", adminUsername)

	// Check if admin user exists
	result, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(usersTable),
		Key: map[string]*dynamodb.AttributeValue{
			"Username": {S: aws.String(adminUsername)},
		},
	})

	if err != nil {
		fmt.Printf("Error checking for admin user: %v\n", err)
		return
	}

	if result.Item == nil {
		fmt.Println("Admin user doesn't exist, creating...")
		// Create admin user
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
		if err != nil {
			fmt.Printf("Error hashing password: %v\n", err)
			return
		}

		item := map[string]*dynamodb.AttributeValue{
			"Username":     {S: aws.String(adminUsername)},
			"PasswordHash": {S: aws.String(string(hashedPassword))},
			"CreatedAt":    {S: aws.String(time.Now().Format(time.RFC3339))},
			"Role":         {S: aws.String("admin")},
		}

		_, err = ddbClient.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String(usersTable),
			Item:      item,
		})

		if err != nil {
			fmt.Printf("Error creating admin user: %v\n", err)
		} else {
			fmt.Println("Admin user created successfully!")
		}
	} else {
		fmt.Println("Admin user already exists")
	}
}

// getImageDate extracts date from EXIF or falls back to InsertedDateTime
func getImageDate(img ImageResponse) time.Time {
	// Try EXIF DateTimeOriginal first
	if dateStr, ok := img.EXIFData["DateTimeOriginal"]; ok {
		cleaned := strings.Trim(dateStr, "\"")
		if t, err := time.Parse("2006:01:02 15:04:05", cleaned); err == nil {
			return t
		}
	}
	// Try DateTime
	if dateStr, ok := img.EXIFData["DateTime"]; ok {
		cleaned := strings.Trim(dateStr, "\"")
		if t, err := time.Parse("2006:01:02 15:04:05", cleaned); err == nil {
			return t
		}
	}
	// Fall back to InsertedDateTime
	if t, err := time.Parse(time.RFC3339, img.InsertedDateTime); err == nil {
		return t
	}
	return time.Now()
}

// buildDatePath returns YYYY/MM/DD format
func buildDatePath(t time.Time) string {
	return fmt.Sprintf("%d/%02d/%02d", t.Year(), int(t.Month()), t.Day())
}

// sanitizeS3Name converts a project name to an S3-safe prefix
// Rules: lowercase, alphanumeric + underscores, no consecutive underscores, max 63 chars
func sanitizeS3Name(name string) string {
	// Convert to lowercase
	result := strings.ToLower(name)

	// Replace spaces with underscores
	result = strings.ReplaceAll(result, " ", "_")

	// Replace any character that's not alphanumeric or underscore with underscore
	reg := regexp.MustCompile(`[^a-z0-9_]`)
	result = reg.ReplaceAllString(result, "_")

	// Replace multiple consecutive underscores with single underscore
	reg = regexp.MustCompile(`_+`)
	result = reg.ReplaceAllString(result, "_")

	// Trim leading/trailing underscores
	result = strings.Trim(result, "_")

	// Limit to 63 characters (S3 prefix component limit)
	if len(result) > 63 {
		result = result[:63]
		result = strings.TrimRight(result, "_")
	}

	// Ensure we have something
	if result == "" {
		result = "project"
	}

	return result
}

// getProjectS3Prefix returns the S3 prefix for a project, preferring S3Prefix if set
func getProjectS3Prefix(project Project) string {
	if project.S3Prefix != "" {
		return project.S3Prefix
	}
	// Fallback to ProjectID for backward compatibility
	return project.ProjectID
}

// Common RAW file extensions
var rawExtensions = []string{
	".cr2", ".cr3", ".nef", ".arw", ".raf", ".orf", ".dng",
	".rw2", ".pef", ".srw", ".3fr", ".raw", ".rwl", ".mrw",
	".nrw", ".kdc", ".dcr", ".sr2", ".erf", ".mef", ".mos",
}

// findRawFiles looks for RAW files with the same base name as the original file
func findRawFiles(bucket string, originalFile string) []string {
	var rawFiles []string

	// Get the directory and base name without extension
	dir := filepath.Dir(originalFile)
	baseName := strings.TrimSuffix(filepath.Base(originalFile), filepath.Ext(originalFile))
	baseNameLower := strings.ToLower(baseName)

	// List objects in the same directory
	prefix := dir + "/"
	listResult, err := s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		fmt.Printf("Warning: failed to list S3 objects for raw file search: %v\n", err)
		return rawFiles
	}

	for _, obj := range listResult.Contents {
		key := *obj.Key
		fileName := filepath.Base(key)
		fileNameLower := strings.ToLower(fileName)
		fileExt := strings.ToLower(filepath.Ext(fileName))
		fileBaseName := strings.TrimSuffix(fileNameLower, fileExt)

		// Skip if it's the original file itself
		if key == originalFile {
			continue
		}

		// Check if base name matches and extension is a RAW format
		if fileBaseName == baseNameLower {
			for _, rawExt := range rawExtensions {
				if fileExt == rawExt {
					fmt.Printf("  Found RAW file: %s\n", key)
					rawFiles = append(rawFiles, key)
					break
				}
			}
		}
	}

	return rawFiles
}

// moveImageFiles moves original, thumbnails, and related files to new location
func moveImageFiles(bucket string, img ImageResponse, destPrefix string) (map[string]string, error) {
	newPaths := make(map[string]string)

	// Move original file
	origFilename := filepath.Base(img.OriginalFile)
	newOriginal := destPrefix + "/" + origFilename
	if err := copyS3Object(bucket, img.OriginalFile, newOriginal); err != nil {
		return nil, fmt.Errorf("failed to copy original: %v", err)
	}
	deleteS3Object(bucket, img.OriginalFile)
	newPaths["original"] = newOriginal

	// Move thumbnails
	thumb50Name := filepath.Base(img.Thumbnail50)
	newThumb50 := destPrefix + "/" + thumb50Name
	copyS3Object(bucket, img.Thumbnail50, newThumb50)
	deleteS3Object(bucket, img.Thumbnail50)
	newPaths["thumbnail50"] = newThumb50

	thumb400Name := filepath.Base(img.Thumbnail400)
	newThumb400 := destPrefix + "/" + thumb400Name
	copyS3Object(bucket, img.Thumbnail400, newThumb400)
	deleteS3Object(bucket, img.Thumbnail400)
	newPaths["thumbnail400"] = newThumb400

	// Find and move RAW files (same base name, different extension)
	rawFiles := findRawFiles(bucket, img.OriginalFile)
	var movedRawFiles []string
	for _, rawFile := range rawFiles {
		rawFilename := filepath.Base(rawFile)
		newRawPath := destPrefix + "/" + rawFilename
		fmt.Printf("  Moving RAW file: %s -> %s\n", rawFile, newRawPath)
		if err := copyS3Object(bucket, rawFile, newRawPath); err != nil {
			fmt.Printf("  Warning: failed to copy RAW file %s: %v\n", rawFile, err)
			continue
		}
		deleteS3Object(bucket, rawFile)
		movedRawFiles = append(movedRawFiles, newRawPath)
	}
	newPaths["rawFiles"] = strings.Join(movedRawFiles, ",")

	// Move any existing related files
	for _, relFile := range img.RelatedFiles {
		relName := filepath.Base(relFile)
		newRelPath := destPrefix + "/" + relName
		copyS3Object(bucket, relFile, newRelPath)
		deleteS3Object(bucket, relFile)
	}

	return newPaths, nil
}

func copyS3Object(bucket, srcKey, dstKey string) error {
	_, err := s3Client.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(url.PathEscape(bucket + "/" + srcKey)),
		Key:        aws.String(dstKey),
	})
	return err
}

func deleteS3Object(bucket, key string) {
	s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
}

// analyzeImageWithGPT4o sends the image to OpenAI GPT-4o for keyword and description generation
func analyzeImageWithGPT4o(bucket, thumbnailKey string) (*AIAnalysisResult, error) {
	if openaiAPIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	// Download thumbnail from S3
	getResult, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(thumbnailKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download thumbnail: %v", err)
	}
	defer getResult.Body.Close()

	imageData, err := io.ReadAll(getResult.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read thumbnail data: %v", err)
	}

	// Determine MIME type from extension
	mimeType := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(thumbnailKey), ".png") {
		mimeType = "image/png"
	}

	// Base64 encode the image
	base64Image := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

	// Build OpenAI request
	prompt := `Analyze this photograph and provide:
1. A list of 10-15 relevant keywords for cataloging (single words or short phrases, lowercase)
2. A brief description (2-3 sentences) describing the image content, style, and mood

Respond in JSON format exactly like this:
{"keywords": ["keyword1", "keyword2", ...], "description": "Your description here."}`

	openaiReq := OpenAIRequest{
		Model: "gpt-4o",
		Messages: []OpenAIMessage{
			{
				Role: "user",
				Content: []interface{}{
					OpenAITextContent{
						Type: "text",
						Text: prompt,
					},
					OpenAIImageContent{
						Type: "image_url",
						ImageURL: OpenAIImageURL{
							URL:    dataURL,
							Detail: "high",
						},
					},
				},
			},
		},
		MaxTokens: 500,
	}

	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI request: %v", err)
	}

	// Make HTTP request to OpenAI
	httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+openaiAPIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAI response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var openaiResp OpenAIResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %v", err)
	}

	if openaiResp.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s", openaiResp.Error.Message)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	// Parse the JSON response from GPT-4o
	content := openaiResp.Choices[0].Message.Content

	// Try to extract JSON from the response (it might be wrapped in markdown code blocks)
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
		// If JSON parsing fails, try to extract what we can
		fmt.Printf("Failed to parse GPT-4o JSON response: %v, content: %s\n", err, content)
		return nil, fmt.Errorf("failed to parse GPT-4o response as JSON: %v", err)
	}

	fmt.Printf("GPT-4o analysis complete: %d keywords, description length: %d\n", len(result.Keywords), len(result.Description))
	return &result, nil
}

// handleAsyncMoveFiles processes async file move requests
func handleAsyncMoveFiles(req AsyncMoveRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	fmt.Printf("Async move started for image %s to %s\n", req.ImageGUID, req.DestPrefix)

	// Update status to "moving"
	_, err := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(req.ImageGUID)},
		},
		UpdateExpression: aws.String("SET MoveStatus = :status"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":status": {S: aws.String("moving")},
		},
	})
	if err != nil {
		fmt.Printf("Error updating move status to moving: %v\n", err)
	}

	// Get current image metadata
	getResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(req.ImageGUID)},
		},
	})
	if err != nil || getResult.Item == nil {
		// Update status to failed
		ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(req.ImageGUID)},
			},
			UpdateExpression: aws.String("SET MoveStatus = :status"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":status": {S: aws.String("failed")},
			},
		})
		return events.APIGatewayProxyResponse{StatusCode: 200, Headers: headers, Body: `{"success": false, "error": "Image not found"}`}, nil
	}

	var img ImageResponse
	dynamodbattribute.UnmarshalMap(getResult.Item, &img)

	// Move the files
	newPaths, err := moveImageFiles(req.Bucket, img, req.DestPrefix)
	if err != nil {
		fmt.Printf("Error moving files: %v\n", err)
		// Update status to failed
		ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(req.ImageGUID)},
			},
			UpdateExpression: aws.String("SET MoveStatus = :status"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":status": {S: aws.String("failed")},
			},
		})
		return events.APIGatewayProxyResponse{StatusCode: 200, Headers: headers, Body: fmt.Sprintf(`{"success": false, "error": "%v"}`, err)}, nil
	}

	// Update DynamoDB with new paths and complete status
	_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(req.ImageGUID)},
		},
		UpdateExpression: aws.String("SET OriginalFile = :orig, Thumbnail50 = :t50, Thumbnail400 = :t400, #status = :newStatus, MoveStatus = :moveStatus, UpdatedDateTime = :updated"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":orig":       {S: aws.String(newPaths["original"])},
			":t50":        {S: aws.String(newPaths["thumbnail50"])},
			":t400":       {S: aws.String(newPaths["thumbnail400"])},
			":newStatus":  {S: aws.String(req.NewStatus)},
			":moveStatus": {S: aws.String("complete")},
			":updated":    {S: aws.String(time.Now().Format(time.RFC3339))},
		},
		ExpressionAttributeNames: map[string]*string{
			"#status": aws.String("Status"),
		},
	})
	if err != nil {
		fmt.Printf("Error updating image after move: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Headers: headers, Body: fmt.Sprintf(`{"success": false, "error": "%v"}`, err)}, nil
	}

	// For approved images, analyze with GPT-4o to generate keywords and description
	if req.NewStatus == "approved" && openaiAPIKey != "" {
		fmt.Printf("Starting GPT-4o analysis for approved image %s\n", req.ImageGUID)

		// Use the new thumbnail path after move
		aiResult, err := analyzeImageWithGPT4o(req.Bucket, newPaths["thumbnail400"])
		if err != nil {
			fmt.Printf("GPT-4o analysis failed for image %s: %v\n", req.ImageGUID, err)
			// Don't fail the whole operation, just log the error
		} else {
			// Merge AI keywords with existing user keywords (case-insensitive deduplication)
			existingKeywordsLower := make(map[string]bool)
			for _, kw := range img.Keywords {
				existingKeywordsLower[strings.ToLower(kw)] = true
			}

			mergedKeywords := append([]string{}, img.Keywords...) // Start with existing
			for _, kw := range aiResult.Keywords {
				if !existingKeywordsLower[strings.ToLower(kw)] {
					mergedKeywords = append(mergedKeywords, kw)
					existingKeywordsLower[strings.ToLower(kw)] = true
				}
			}

			// Build update expression for keywords and description
			updateExpr := "SET #desc = :desc, UpdatedDateTime = :updated"
			exprAttrValues := map[string]*dynamodb.AttributeValue{
				":desc":    {S: aws.String(aiResult.Description)},
				":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
			}
			exprAttrNames := map[string]*string{
				"#desc": aws.String("Description"),
			}

			if len(mergedKeywords) > 0 {
				keywordsList := make([]*dynamodb.AttributeValue, len(mergedKeywords))
				for i, kw := range mergedKeywords {
					keywordsList[i] = &dynamodb.AttributeValue{S: aws.String(kw)}
				}
				updateExpr += ", Keywords = :keywords"
				exprAttrValues[":keywords"] = &dynamodb.AttributeValue{L: keywordsList}
			}

			_, updateErr := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
				TableName: aws.String(imageTable),
				Key: map[string]*dynamodb.AttributeValue{
					"ImageGUID": {S: aws.String(req.ImageGUID)},
				},
				UpdateExpression:          aws.String(updateExpr),
				ExpressionAttributeValues: exprAttrValues,
				ExpressionAttributeNames:  exprAttrNames,
			})
			if updateErr != nil {
				fmt.Printf("Error updating image with AI analysis: %v\n", updateErr)
			} else {
				fmt.Printf("GPT-4o analysis saved for image %s: %d keywords\n", req.ImageGUID, len(mergedKeywords))
			}
		}
	}

	fmt.Printf("Async move completed for image %s\n", req.ImageGUID)
	return events.APIGatewayProxyResponse{StatusCode: 200, Headers: headers, Body: `{"success": true}`}, nil
}

// triggerAsyncMove invokes Lambda asynchronously to move files
func triggerAsyncMove(imageGUID, destPrefix, newStatus, bucket string) error {
	payload, err := json.Marshal(AsyncMoveRequest{
		Action:     "move_files",
		ImageGUID:  imageGUID,
		DestPrefix: destPrefix,
		NewStatus:  newStatus,
		Bucket:     bucket,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal async request: %v", err)
	}

	// Wrap payload in API Gateway format for the handler
	wrappedPayload, _ := json.Marshal(map[string]string{
		"body": string(payload),
	})

	_, err = lambdaClient.Invoke(&lambdasvc.InvokeInput{
		FunctionName:   aws.String(functionName),
		InvocationType: aws.String("Event"), // Async invocation
		Payload:        wrappedPayload,
	})
	if err != nil {
		return fmt.Errorf("failed to invoke lambda: %v", err)
	}

	return nil
}

func getColorName(groupNumber int) string {
	// Matches Lightroom color labels: Red, Yellow, Green, Blue, Purple
	colors := map[int]string{
		0: "none", 1: "red", 2: "yellow", 3: "green", 4: "blue", 5: "purple",
	}
	if name, ok := colors[groupNumber]; ok {
		return name
	}
	return "none"
}

// getLightroomColorLabel maps our group numbers to Lightroom color labels
func getLightroomColorLabel(groupNumber int) string {
	// Lightroom color labels: Red, Yellow, Green, Blue, Purple
	labels := map[int]string{
		1: "Red",
		2: "Yellow",
		3: "Green",
		4: "Blue",
		5: "Purple",
	}
	if label, ok := labels[groupNumber]; ok {
		return label
	}
	return "" // No color label
}

// generateProjectCatalog creates or updates a Lightroom catalog for a project
func generateProjectCatalog(project Project, images []ImageResponse) error {
	// Create catalog file in /tmp
	catalogPath := fmt.Sprintf("/tmp/%s.lrcat", project.ProjectID)

	// Remove existing catalog if present
	os.Remove(catalogPath)

	// Create new catalog
	catalog, err := lrcat.NewCatalog(catalogPath)
	if err != nil {
		return fmt.Errorf("failed to create catalog: %v", err)
	}
	defer catalog.Close()

	// Track root folders and subfolders we've created
	rootFolders := make(map[string]int64) // path -> ID

	// Process each image
	for _, img := range images {
		// Get the directory path from the original file
		dirPath := filepath.Dir(img.OriginalFile)
		fileName := filepath.Base(img.OriginalFile)

		// Determine root folder (first path component after projects/<name>/)
		// e.g., "projects/MyProject/2024/01/15/image.jpg" -> root="/projects/MyProject", subfolder="2024/01/15"
		parts := strings.Split(dirPath, "/")
		var rootPath, subPath string

		if len(parts) >= 2 && parts[0] == "projects" {
			// Root is "projects/<name>"
			rootPath = "/" + parts[0] + "/" + parts[1]
			if len(parts) > 2 {
				subPath = strings.Join(parts[2:], "/")
			}
		} else {
			// Fallback: use full directory as root
			rootPath = "/" + dirPath
			subPath = ""
		}

		// Get or create root folder
		rootID, exists := rootFolders[rootPath]
		if !exists {
			rootFolder, err := catalog.AddRootFolder(rootPath)
			if err != nil {
				fmt.Printf("Warning: failed to add root folder %s: %v\n", rootPath, err)
				continue
			}
			rootID = rootFolder.ID
			rootFolders[rootPath] = rootID
		}

		// Get or create subfolder
		var folderID int64 = rootID
		if subPath != "" {
			folder, err := catalog.GetOrCreateFolder(rootID, subPath)
			if err != nil {
				fmt.Printf("Warning: failed to create subfolder %s: %v\n", subPath, err)
			} else {
				folderID = folder.ID
			}
		}
		_ = folderID // folderID used by AddImage automatically via path

		// Parse capture time from EXIF
		captureTime := getImageDate(img)

		// Build image input for lrcat
		imageInput := &lrcat.ImageInput{
			FilePath:    "/" + img.OriginalFile, // Make absolute path
			CaptureTime: captureTime,
			FileFormat:  detectFileFormat(fileName),
			ColorLabel:  getLightroomColorLabel(img.GroupNumber),
			Pick:        1, // All project images are "picked"
		}

		// Set dimensions if available
		if img.Width > 0 {
			w := img.Width
			imageInput.Width = &w
		}
		if img.Height > 0 {
			h := img.Height
			imageInput.Height = &h
		}

		// Add rating from image (1-5 stars)
		if img.Rating > 0 && img.Rating <= 5 {
			rating := img.Rating
			imageInput.Rating = &rating
		}

		// Add image to catalog
		catalogImage, err := catalog.AddImage(imageInput)
		if err != nil {
			fmt.Printf("Warning: failed to add image %s: %v\n", fileName, err)
			continue
		}

		// Add keywords to the image
		if len(img.Keywords) > 0 {
			for _, keyword := range img.Keywords {
				kw, err := catalog.GetOrCreateKeyword(keyword, nil)
				if err != nil {
					fmt.Printf("Warning: failed to create keyword %s: %v\n", keyword, err)
					continue
				}
				if err := catalog.AddKeywordToImage(catalogImage.ID, kw.ID); err != nil {
					fmt.Printf("Warning: failed to add keyword %s to image: %v\n", keyword, err)
				}
			}
		}
	}

	// Close catalog before uploading
	catalog.Close()

	// Upload catalog to S3
	catalogFile, err := os.Open(catalogPath)
	if err != nil {
		return fmt.Errorf("failed to open catalog for upload: %v", err)
	}
	defer catalogFile.Close()

	// Read file content
	catalogData, err := io.ReadAll(catalogFile)
	if err != nil {
		return fmt.Errorf("failed to read catalog: %v", err)
	}

	// Upload to S3 at projects/<s3Prefix>/project.lrcat (use sanitized name)
	s3Prefix := getProjectS3Prefix(project)
	s3Key := fmt.Sprintf("projects/%s/project.lrcat", s3Prefix)
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(s3Key),
		Body:        bytes.NewReader(catalogData),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload catalog to S3: %v", err)
	}

	// Update project record with catalog path
	ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(project.ProjectID)},
		},
		UpdateExpression: aws.String("SET CatalogPath = :path, CatalogUpdatedAt = :updated"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":path":    {S: aws.String(s3Key)},
			":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
		},
	})

	// Clean up temp file
	os.Remove(catalogPath)

	fmt.Printf("Generated catalog for project %s with %d images at %s\n", project.Name, len(images), s3Key)
	return nil
}

// detectFileFormat returns the file format for lrcat based on extension
func detectFileFormat(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "JPG"
	case ".png":
		return "PNG"
	case ".tif", ".tiff":
		return "TIFF"
	case ".raw", ".cr2", ".cr3", ".nef", ".arw", ".dng":
		return "RAW"
	case ".heic", ".heif":
		return "HEIC"
	default:
		return "JPG"
	}
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	headers := map[string]string{
		"Content-Type":                "application/json",
		"Access-Control-Allow-Origin": "*",
		"Access-Control-Allow-Headers": "Content-Type,Authorization",
		"Access-Control-Allow-Methods": "GET,POST,PUT,DELETE,OPTIONS",
	}

	// Check if this is an async move request (direct Lambda invocation)
	if request.Body != "" && request.HTTPMethod == "" && request.Path == "" {
		var asyncReq AsyncMoveRequest
		if err := json.Unmarshal([]byte(request.Body), &asyncReq); err == nil && asyncReq.Action == "move_files" {
			return handleAsyncMoveFiles(asyncReq, headers)
		}
	}

	// Handle OPTIONS requests for CORS
	if request.HTTPMethod == "OPTIONS" {
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Headers:    headers,
		}, nil
	}

	path := request.Path
	method := request.HTTPMethod

	// Login endpoint doesn't require authentication
	if path == "/api/login" && method == "POST" {
		return handleLogin(request, headers)
	}

	// All other endpoints require authentication
	token := extractToken(request.Headers)
	if token == "" {
		return errorResponse(401, "Unauthorized", headers)
	}

	if !validateToken(token) {
		return errorResponse(401, "Invalid token", headers)
	}

	// Route requests
	switch {
	case path == "/api/images" && method == "GET":
		return handleListImages(request, headers)
	case strings.HasPrefix(path, "/api/images/") && method == "PUT":
		imageID := strings.TrimPrefix(path, "/api/images/")
		return handleUpdateImage(imageID, request, headers)
	case strings.HasPrefix(path, "/api/images/") && strings.HasSuffix(path, "/download") && method == "GET":
		imageID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/images/"), "/download")
		return handleDownload(imageID, headers)
	case strings.HasPrefix(path, "/api/images/") && strings.HasSuffix(path, "/regenerate-ai") && method == "POST":
		imageID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/images/"), "/regenerate-ai")
		return handleRegenerateAI(imageID, headers)
	case strings.HasPrefix(path, "/api/images/") && strings.HasSuffix(path, "/undelete") && method == "POST":
		imageID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/images/"), "/undelete")
		return handleUndeleteImage(imageID, headers)
	case strings.HasPrefix(path, "/api/images/") && method == "DELETE":
		imageID := strings.TrimPrefix(path, "/api/images/")
		return handleDeleteImage(imageID, headers)
	// Project routes
	case path == "/api/projects" && method == "GET":
		return handleListProjects(headers)
	case path == "/api/projects" && method == "POST":
		return handleCreateProject(request, headers)
	case strings.HasPrefix(path, "/api/projects/") && !strings.Contains(path[len("/api/projects/"):], "/") && method == "PUT":
		projectID := strings.TrimPrefix(path, "/api/projects/")
		return handleUpdateProject(projectID, request, headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.HasSuffix(path, "/images") && method == "POST":
		projectID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/projects/"), "/images")
		return handleAddToProject(projectID, request, headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.HasSuffix(path, "/images") && method == "GET":
		projectID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/projects/"), "/images")
		return handleGetProjectImages(projectID, headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.HasSuffix(path, "/catalog") && method == "GET":
		projectID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/projects/"), "/catalog")
		return handleGetProjectCatalog(projectID, headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.HasSuffix(path, "/generate-zip") && method == "POST":
		projectID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/projects/"), "/generate-zip")
		return handleGenerateZip(projectID, headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.Contains(path, "/zips/") && strings.HasSuffix(path, "/download") && method == "GET":
		// Extract projectID and zipKey from path: /api/projects/{projectId}/zips/{zipKey}/download
		parts := strings.Split(path, "/")
		if len(parts) >= 7 {
			projectID := parts[3]
			zipKeyEncoded := strings.Join(parts[5:len(parts)-1], "/")
			// URL-decode the zipKey since it may contain encoded slashes
			zipKey, err := url.QueryUnescape(zipKeyEncoded)
			if err != nil {
				zipKey = zipKeyEncoded // Fall back to encoded version if decode fails
			}
			return handleGetZipDownload(projectID, zipKey, headers)
		}
		return errorResponse(400, "Invalid zip download path", headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.HasSuffix(path, "/zip-logs") && method == "GET":
		projectID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/projects/"), "/zip-logs")
		return handleGetZipLogs(projectID, headers)
	default:
		return errorResponse(404, "Not found", headers)
	}
}

func handleLogin(request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	var loginReq LoginRequest
	if err := json.Unmarshal([]byte(request.Body), &loginReq); err != nil {
		return errorResponse(400, "Invalid request body", headers)
	}

	// Get user from DynamoDB
	result, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(usersTable),
		Key: map[string]*dynamodb.AttributeValue{
			"Username": {S: aws.String(loginReq.Username)},
		},
	})

	if err != nil || result.Item == nil {
		return errorResponse(401, "Invalid credentials", headers)
	}

	var passwordHash string
	if result.Item["PasswordHash"] != nil {
		passwordHash = *result.Item["PasswordHash"].S
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(loginReq.Password)); err != nil {
		return errorResponse(401, "Invalid credentials", headers)
	}

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": loginReq.Username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return errorResponse(500, "Failed to generate token", headers)
	}

	response := LoginResponse{Token: tokenString}
	body, _ := json.Marshal(response)

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleListImages(request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get filter parameters from query string
	stateFilter := request.QueryStringParameters["state"]
	groupFilter := request.QueryStringParameters["group"]

	// Default to unreviewed if no state specified
	if stateFilter == "" {
		stateFilter = "unreviewed"
	}

	var result *dynamodb.QueryOutput
	var err error

	// Determine reviewed value based on state filter
	switch stateFilter {
	case "unreviewed":
		// Query for unreviewed images using GSI
		input := &dynamodb.QueryInput{
			TableName:              aws.String(imageTable),
			IndexName:              aws.String("ReviewedIndex"),
			KeyConditionExpression: aws.String("Reviewed = :reviewed"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":reviewed": {S: aws.String("false")},
			},
		}
		result, err = ddbClient.Query(input)
	case "approved", "rejected":
		// Query for reviewed images using GSI
		input := &dynamodb.QueryInput{
			TableName:              aws.String(imageTable),
			IndexName:              aws.String("ReviewedIndex"),
			KeyConditionExpression: aws.String("Reviewed = :reviewed"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":reviewed": {S: aws.String("true")},
			},
		}
		result, err = ddbClient.Query(input)
	case "deleted":
		// Scan for deleted images (Status = "deleted")
		scanResult, scanErr := ddbClient.Scan(&dynamodb.ScanInput{
			TableName:        aws.String(imageTable),
			FilterExpression: aws.String("#status = :deleted"),
			ExpressionAttributeNames: map[string]*string{
				"#status": aws.String("Status"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":deleted": {S: aws.String("deleted")},
			},
		})
		if scanErr != nil {
			err = scanErr
		} else {
			result = &dynamodb.QueryOutput{Items: scanResult.Items}
		}
	case "all":
		// Scan all images (excluding deleted)
		scanResult, scanErr := ddbClient.Scan(&dynamodb.ScanInput{
			TableName:        aws.String(imageTable),
			FilterExpression: aws.String("attribute_not_exists(#status) OR #status <> :deleted"),
			ExpressionAttributeNames: map[string]*string{
				"#status": aws.String("Status"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":deleted": {S: aws.String("deleted")},
			},
		})
		if scanErr != nil {
			err = scanErr
		} else {
			result = &dynamodb.QueryOutput{Items: scanResult.Items}
		}
	default:
		return errorResponse(400, "Invalid state filter", headers)
	}

	if err != nil {
		fmt.Printf("Error querying images: %v\n", err)
		return errorResponse(500, "Failed to list images", headers)
	}

	// Initialize as empty slice to ensure JSON returns [] instead of null
	images := make([]ImageResponse, 0)
	for _, item := range result.Items {
		var img ImageResponse
		dynamodbattribute.UnmarshalMap(item, &img)

		// Skip deleted images unless specifically querying for them
		if stateFilter != "deleted" && img.Status == "deleted" {
			continue
		}

		// Filter by state (approved vs rejected)
		if stateFilter == "approved" && img.GroupNumber == 0 {
			continue // Skip rejected (no group assigned)
		}
		if stateFilter == "rejected" && img.GroupNumber > 0 {
			continue // Skip approved (has group assigned)
		}

		// Filter by group if specified
		if groupFilter != "" && groupFilter != "all" {
			groupNum := 0
			fmt.Sscanf(groupFilter, "%d", &groupNum)
			if img.GroupNumber != groupNum {
				continue
			}
		}

		images = append(images, img)
	}

	body, _ := json.Marshal(images)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleUpdateImage(imageID string, request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	var updateReq UpdateImageRequest
	if err := json.Unmarshal([]byte(request.Body), &updateReq); err != nil {
		return errorResponse(400, "Invalid request body", headers)
	}

	// Get current image metadata to check if this is a new review
	getResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
	})
	if err != nil || getResult.Item == nil {
		return errorResponse(404, "Image not found", headers)
	}

	var img ImageResponse
	dynamodbattribute.UnmarshalMap(getResult.Item, &img)

	// Check if this is a new review (moving from unreviewed to reviewed)
	var triggerMove bool
	var destPrefix string
	var newStatus string

	if updateReq.Reviewed == "true" && img.Reviewed == "false" {
		imageDate := getImageDate(img)
		datePath := buildDatePath(imageDate)

		if updateReq.GroupNumber > 0 {
			// Approved with a group - move to approved/<color>/YYYY/MM/DD
			colorName := getColorName(updateReq.GroupNumber)
			destPrefix = fmt.Sprintf("approved/%s/%s", colorName, datePath)
			newStatus = "approved"
			triggerMove = true
		} else {
			// Rejected (group 0) - move to rejected/YYYY/MM/DD
			destPrefix = "rejected/" + datePath
			newStatus = "rejected"
			triggerMove = true
		}
	}

	// Build update expression
	updateExpr := "SET GroupNumber = :group, UpdatedDateTime = :updated"
	exprAttrValues := map[string]*dynamodb.AttributeValue{
		":group":   {N: aws.String(fmt.Sprintf("%d", updateReq.GroupNumber))},
		":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
	}
	exprAttrNames := make(map[string]*string)

	if updateReq.ColorCode != "" {
		updateExpr += ", ColorCode = :color"
		exprAttrValues[":color"] = &dynamodb.AttributeValue{S: aws.String(updateReq.ColorCode)}
	}

	if updateReq.Promoted {
		updateExpr += ", Promoted = :promoted"
		exprAttrValues[":promoted"] = &dynamodb.AttributeValue{BOOL: aws.Bool(true)}
	}

	// Rating is 0-5, where 0 means no rating
	if updateReq.Rating >= 0 && updateReq.Rating <= 5 {
		updateExpr += ", Rating = :rating"
		exprAttrValues[":rating"] = &dynamodb.AttributeValue{N: aws.String(fmt.Sprintf("%d", updateReq.Rating))}
	}

	// Update keywords if provided
	if updateReq.Keywords != nil {
		if len(updateReq.Keywords) > 0 {
			keywordsList := make([]*dynamodb.AttributeValue, len(updateReq.Keywords))
			for i, kw := range updateReq.Keywords {
				keywordsList[i] = &dynamodb.AttributeValue{S: aws.String(kw)}
			}
			updateExpr += ", Keywords = :keywords"
			exprAttrValues[":keywords"] = &dynamodb.AttributeValue{L: keywordsList}
		} else {
			// Empty array means remove keywords
			updateExpr += " REMOVE Keywords"
		}
	}

	if updateReq.Reviewed != "" {
		updateExpr += ", Reviewed = :reviewed"
		exprAttrValues[":reviewed"] = &dynamodb.AttributeValue{S: aws.String(updateReq.Reviewed)}
	}

	// Set move status to pending if we need to trigger async move
	if triggerMove {
		updateExpr += ", MoveStatus = :moveStatus"
		exprAttrValues[":moveStatus"] = &dynamodb.AttributeValue{S: aws.String("pending")}
	}

	// Update the image metadata
	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprAttrValues,
	}
	if len(exprAttrNames) > 0 {
		updateInput.ExpressionAttributeNames = exprAttrNames
	}

	_, err = ddbClient.UpdateItem(updateInput)
	if err != nil {
		fmt.Printf("Error updating image: %v\n", err)
		return errorResponse(500, "Failed to update image", headers)
	}

	// Store review decision
	reviewID := fmt.Sprintf("review_%d", time.Now().Unix())
	reviewItem := map[string]*dynamodb.AttributeValue{
		"ReviewID":    {S: aws.String(reviewID)},
		"ImageGUID":   {S: aws.String(imageID)},
		"GroupNumber": {N: aws.String(fmt.Sprintf("%d", updateReq.GroupNumber))},
		"ColorCode":   {S: aws.String(updateReq.ColorCode)},
		"Promoted":    {BOOL: aws.Bool(updateReq.Promoted)},
		"Timestamp":   {S: aws.String(time.Now().Format(time.RFC3339))},
	}

	ddbClient.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(reviewGroupsTable),
		Item:      reviewItem,
	})

	// Trigger async file move if needed
	if triggerMove {
		if err := triggerAsyncMove(imageID, destPrefix, newStatus, bucketName); err != nil {
			fmt.Printf("Error triggering async move: %v\n", err)
			// Don't fail the request - the move status will show the issue
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       `{"success": true}`,
	}, nil
}

func handleRegenerateAI(imageID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Check if OpenAI API key is configured
	if openaiAPIKey == "" {
		return errorResponse(400, "AI content generation is not configured", headers)
	}

	// Get image metadata
	result, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
	})

	if err != nil || result.Item == nil {
		return errorResponse(404, "Image not found", headers)
	}

	var img ImageResponse
	dynamodbattribute.UnmarshalMap(result.Item, &img)

	// Call GPT-4o to analyze the image
	aiResult, err := analyzeImageWithGPT4o(img.Bucket, img.Thumbnail400)
	if err != nil {
		fmt.Printf("Error analyzing image with GPT-4o: %v\n", err)
		return errorResponse(500, fmt.Sprintf("Failed to analyze image: %v", err), headers)
	}

	// Merge AI keywords with existing user keywords (case-insensitive deduplication)
	existingKeywordsLower := make(map[string]bool)
	for _, kw := range img.Keywords {
		existingKeywordsLower[strings.ToLower(kw)] = true
	}

	mergedKeywords := append([]string{}, img.Keywords...) // Start with existing
	for _, kw := range aiResult.Keywords {
		if !existingKeywordsLower[strings.ToLower(kw)] {
			mergedKeywords = append(mergedKeywords, kw)
			existingKeywordsLower[strings.ToLower(kw)] = true
		}
	}

	// Build update expression for keywords and description
	updateExpr := "SET UpdatedDateTime = :updated"
	exprAttrValues := map[string]*dynamodb.AttributeValue{
		":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
	}

	if len(mergedKeywords) > 0 {
		keywordsList := make([]*dynamodb.AttributeValue, len(mergedKeywords))
		for i, kw := range mergedKeywords {
			keywordsList[i] = &dynamodb.AttributeValue{S: aws.String(kw)}
		}
		updateExpr += ", Keywords = :keywords"
		exprAttrValues[":keywords"] = &dynamodb.AttributeValue{L: keywordsList}
	}

	if aiResult.Description != "" {
		updateExpr += ", Description = :description"
		exprAttrValues[":description"] = &dynamodb.AttributeValue{S: aws.String(aiResult.Description)}
	}

	// Update the image in DynamoDB
	_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprAttrValues,
	})
	if err != nil {
		fmt.Printf("Error updating image with AI content: %v\n", err)
		return errorResponse(500, "Failed to save AI content", headers)
	}

	// Return the updated content
	response := map[string]interface{}{
		"success":     true,
		"keywords":    mergedKeywords,
		"description": aiResult.Description,
	}
	body, _ := json.Marshal(response)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleDownload(imageID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get image metadata
	result, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
	})

	if err != nil || result.Item == nil {
		return errorResponse(404, "Image not found", headers)
	}

	var img ImageResponse
	dynamodbattribute.UnmarshalMap(result.Item, &img)

	// Generate presigned URL
	req, _ := s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(img.OriginalFile),
	})

	url, err := req.Presign(15 * time.Minute)
	if err != nil {
		return errorResponse(500, "Failed to generate download URL", headers)
	}

	body, _ := json.Marshal(map[string]string{"url": url})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleDeleteImage(imageID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get image metadata first
	result, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
	})

	if err != nil || result.Item == nil {
		return errorResponse(404, "Image not found", headers)
	}

	var img ImageResponse
	dynamodbattribute.UnmarshalMap(result.Item, &img)

	// Get date from EXIF for folder structure
	imageDate := getImageDate(img)
	datePath := buildDatePath(imageDate)
	destPrefix := "deleted/" + datePath

	// Move all files to deleted folder
	newPaths, err := moveImageFiles(bucketName, img, destPrefix)
	if err != nil {
		fmt.Printf("Error moving files: %v\n", err)
		return errorResponse(500, fmt.Sprintf("Failed to move files: %v", err), headers)
	}

	// Update DynamoDB with new paths and status (instead of deleting)
	_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
		UpdateExpression: aws.String("SET OriginalFile = :orig, Thumbnail50 = :t50, Thumbnail400 = :t400, #status = :status, UpdatedDateTime = :updated"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":orig":    {S: aws.String(newPaths["original"])},
			":t50":     {S: aws.String(newPaths["thumbnail50"])},
			":t400":    {S: aws.String(newPaths["thumbnail400"])},
			":status":  {S: aws.String("deleted")},
			":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
		},
		ExpressionAttributeNames: map[string]*string{
			"#status": aws.String("Status"),
		},
	})

	if err != nil {
		fmt.Printf("Error updating metadata: %v\n", err)
		return errorResponse(500, "Failed to update metadata", headers)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       `{"success": true}`,
	}, nil
}

func handleUndeleteImage(imageID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get image metadata first
	result, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
	})

	if err != nil || result.Item == nil {
		return errorResponse(404, "Image not found", headers)
	}

	var img ImageResponse
	dynamodbattribute.UnmarshalMap(result.Item, &img)

	// Verify image is deleted
	if img.Status != "deleted" {
		return errorResponse(400, "Image is not deleted", headers)
	}

	// Get date from EXIF for folder structure
	imageDate := getImageDate(img)
	datePath := buildDatePath(imageDate)
	destPrefix := "inbox/" + datePath

	// Move all files back to inbox folder
	newPaths, err := moveImageFiles(bucketName, img, destPrefix)
	if err != nil {
		fmt.Printf("Error moving files: %v\n", err)
		return errorResponse(500, fmt.Sprintf("Failed to move files: %v", err), headers)
	}

	// Update DynamoDB with new paths and reset status
	_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
		UpdateExpression: aws.String("SET OriginalFile = :orig, Thumbnail50 = :t50, Thumbnail400 = :t400, Reviewed = :reviewed, UpdatedDateTime = :updated REMOVE #status"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":orig":     {S: aws.String(newPaths["original"])},
			":t50":      {S: aws.String(newPaths["thumbnail50"])},
			":t400":     {S: aws.String(newPaths["thumbnail400"])},
			":reviewed": {S: aws.String("false")},
			":updated":  {S: aws.String(time.Now().Format(time.RFC3339))},
		},
		ExpressionAttributeNames: map[string]*string{
			"#status": aws.String("Status"),
		},
	})

	if err != nil {
		fmt.Printf("Error updating metadata: %v\n", err)
		return errorResponse(500, "Failed to update metadata", headers)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       `{"success": true}`,
	}, nil
}

// Project handlers

func handleListProjects(headers map[string]string) (events.APIGatewayProxyResponse, error) {
	result, err := ddbClient.Scan(&dynamodb.ScanInput{
		TableName: aws.String(projectsTable),
	})
	if err != nil {
		fmt.Printf("Error listing projects: %v\n", err)
		return errorResponse(500, "Failed to list projects", headers)
	}

	projects := make([]Project, 0)
	for _, item := range result.Items {
		var p Project
		dynamodbattribute.UnmarshalMap(item, &p)
		projects = append(projects, p)
	}

	body, _ := json.Marshal(projects)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleCreateProject(request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	var req CreateProjectRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return errorResponse(400, "Invalid request body", headers)
	}

	if req.Name == "" {
		return errorResponse(400, "Project name is required", headers)
	}

	// Generate S3-safe prefix from project name
	s3Prefix := sanitizeS3Name(req.Name)

	project := Project{
		ProjectID:  uuid.New().String(),
		Name:       req.Name,
		S3Prefix:   s3Prefix,
		CreatedAt:  time.Now().Format(time.RFC3339),
		ImageCount: 0,
		Keywords:   req.Keywords,
	}

	av, _ := dynamodbattribute.MarshalMap(project)
	_, err := ddbClient.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(projectsTable),
		Item:      av,
	})
	if err != nil {
		fmt.Printf("Error creating project: %v\n", err)
		return errorResponse(500, "Failed to create project", headers)
	}

	// Generate initial empty Lightroom catalog
	if err := generateProjectCatalog(project, []ImageResponse{}); err != nil {
		fmt.Printf("Warning: failed to generate initial catalog: %v\n", err)
		// Don't fail project creation if catalog generation fails
	} else {
		project.CatalogPath = fmt.Sprintf("projects/%s/project.lrcat", s3Prefix)
		project.CatalogUpdatedAt = time.Now().Format(time.RFC3339)
	}

	body, _ := json.Marshal(project)
	return events.APIGatewayProxyResponse{
		StatusCode: 201,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleUpdateProject(projectID string, request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	var req UpdateProjectRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return errorResponse(400, "Invalid request body", headers)
	}

	// Get existing project
	getResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || getResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(getResult.Item, &project)

	// Build update expression
	updateExpr := "SET UpdatedAt = :updated"
	exprAttrValues := map[string]*dynamodb.AttributeValue{
		":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
	}

	// Update name if provided
	if req.Name != "" {
		updateExpr += ", #name = :name"
		exprAttrValues[":name"] = &dynamodb.AttributeValue{S: aws.String(req.Name)}
		project.Name = req.Name
	}

	// Update keywords (can be empty array to clear)
	if req.Keywords != nil {
		if len(req.Keywords) > 0 {
			keywordsList := make([]*dynamodb.AttributeValue, len(req.Keywords))
			for i, kw := range req.Keywords {
				keywordsList[i] = &dynamodb.AttributeValue{S: aws.String(kw)}
			}
			updateExpr += ", Keywords = :keywords"
			exprAttrValues[":keywords"] = &dynamodb.AttributeValue{L: keywordsList}
		} else {
			// Remove keywords if empty array
			updateExpr += " REMOVE Keywords"
		}
		project.Keywords = req.Keywords
	}

	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprAttrValues,
	}

	// Add expression attribute names if we're updating name
	if req.Name != "" {
		updateInput.ExpressionAttributeNames = map[string]*string{
			"#name": aws.String("Name"),
		}
	}

	_, err = ddbClient.UpdateItem(updateInput)
	if err != nil {
		fmt.Printf("Error updating project: %v\n", err)
		return errorResponse(500, "Failed to update project", headers)
	}

	body, _ := json.Marshal(project)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleAddToProject(projectID string, request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	var req AddToProjectRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return errorResponse(400, "Invalid request body", headers)
	}

	// Get project to verify it exists
	projResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || projResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(projResult.Item, &project)

	// Query approved images (Status = 'approved')
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(imageTable),
		IndexName:              aws.String("StatusIndex"),
		KeyConditionExpression: aws.String("#status = :status"),
		ExpressionAttributeNames: map[string]*string{
			"#status": aws.String("Status"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":status": {S: aws.String("approved")},
		},
	}

	// Add group filter if not "all"
	if !req.All && req.Group > 0 {
		queryInput.FilterExpression = aws.String("GroupNumber = :group")
		queryInput.ExpressionAttributeValues[":group"] = &dynamodb.AttributeValue{
			N: aws.String(fmt.Sprintf("%d", req.Group)),
		}
	}

	result, err := ddbClient.Query(queryInput)
	if err != nil {
		fmt.Printf("Error querying approved images: %v\n", err)
		return errorResponse(500, "Failed to query images", headers)
	}

	// Move each image to project folder using sanitized S3 prefix
	s3Prefix := getProjectS3Prefix(project)
	movedCount := 0
	for _, item := range result.Items {
		var img ImageResponse
		dynamodbattribute.UnmarshalMap(item, &img)

		imageDate := getImageDate(img)
		datePath := buildDatePath(imageDate)
		destPrefix := fmt.Sprintf("projects/%s/%s", s3Prefix, datePath)

		newPaths, err := moveImageFiles(bucketName, img, destPrefix)
		if err != nil {
			fmt.Printf("Failed to move image %s: %v\n", img.ImageGUID, err)
			continue
		}

		// Build list of raw files for DynamoDB
		var rawFilesList []*dynamodb.AttributeValue
		if rawFilesStr := newPaths["rawFiles"]; rawFilesStr != "" {
			for _, rf := range strings.Split(rawFilesStr, ",") {
				if rf != "" {
					rawFilesList = append(rawFilesList, &dynamodb.AttributeValue{S: aws.String(rf)})
				}
			}
		}

		// Update image record
		updateExpr := "SET OriginalFile = :orig, Thumbnail50 = :t50, Thumbnail400 = :t400, #status = :status, ProjectID = :proj, UpdatedDateTime = :updated"
		exprValues := map[string]*dynamodb.AttributeValue{
			":orig":    {S: aws.String(newPaths["original"])},
			":t50":     {S: aws.String(newPaths["thumbnail50"])},
			":t400":    {S: aws.String(newPaths["thumbnail400"])},
			":status":  {S: aws.String("project")},
			":proj":    {S: aws.String(projectID)},
			":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
		}

		// Add RelatedFiles if we found any RAW files
		if len(rawFilesList) > 0 {
			updateExpr += ", RelatedFiles = :rawFiles"
			exprValues[":rawFiles"] = &dynamodb.AttributeValue{L: rawFilesList}
		}

		_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(img.ImageGUID)},
			},
			UpdateExpression: aws.String(updateExpr),
			ExpressionAttributeValues: exprValues,
			ExpressionAttributeNames: map[string]*string{
				"#status": aws.String("Status"),
			},
		})
		if err != nil {
			fmt.Printf("Failed to update image record %s: %v\n", img.ImageGUID, err)
			continue
		}
		movedCount++
	}

	// Update project image count
	ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
		UpdateExpression: aws.String("ADD ImageCount :count"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":count": {N: aws.String(fmt.Sprintf("%d", movedCount))},
		},
	})

	// Regenerate Lightroom catalog with all project images
	if movedCount > 0 {
		// Query all images in this project for catalog generation
		projectImagesResult, err := ddbClient.Query(&dynamodb.QueryInput{
			TableName:              aws.String(imageTable),
			IndexName:              aws.String("ProjectIndex"),
			KeyConditionExpression: aws.String("ProjectID = :pid"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":pid": {S: aws.String(projectID)},
			},
		})
		if err == nil {
			var allProjectImages []ImageResponse
			for _, item := range projectImagesResult.Items {
				var img ImageResponse
				dynamodbattribute.UnmarshalMap(item, &img)
				allProjectImages = append(allProjectImages, img)
			}

			// Update project image count for catalog generation
			project.ImageCount += movedCount

			if err := generateProjectCatalog(project, allProjectImages); err != nil {
				fmt.Printf("Warning: failed to regenerate catalog: %v\n", err)
			}
		}
	}

	body, _ := json.Marshal(map[string]int{"movedCount": movedCount})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleGetProjectImages(projectID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	result, err := ddbClient.Query(&dynamodb.QueryInput{
		TableName:              aws.String(imageTable),
		IndexName:              aws.String("ProjectIndex"),
		KeyConditionExpression: aws.String("ProjectID = :pid"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":pid": {S: aws.String(projectID)},
		},
	})
	if err != nil {
		fmt.Printf("Error querying project images: %v\n", err)
		return errorResponse(500, "Failed to query images", headers)
	}

	images := make([]ImageResponse, 0)
	for _, item := range result.Items {
		var img ImageResponse
		dynamodbattribute.UnmarshalMap(item, &img)
		images = append(images, img)
	}

	body, _ := json.Marshal(images)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleGetProjectCatalog(projectID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get project to get catalog path
	projResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || projResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(projResult.Item, &project)

	if project.CatalogPath == "" {
		return errorResponse(404, "Catalog not found for this project", headers)
	}

	// Generate presigned URL for catalog download
	req, _ := s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket:                     aws.String(bucketName),
		Key:                        aws.String(project.CatalogPath),
		ResponseContentDisposition: aws.String(fmt.Sprintf("attachment; filename=\"%s.lrcat\"", project.Name)),
	})

	url, err := req.Presign(15 * time.Minute)
	if err != nil {
		return errorResponse(500, "Failed to generate download URL", headers)
	}

	body, _ := json.Marshal(map[string]string{
		"url":       url,
		"filename":  project.Name + ".lrcat",
		"updatedAt": project.CatalogUpdatedAt,
	})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleGenerateZip(projectID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	if zipLambdaName == "" {
		return errorResponse(500, "Zip generation not configured", headers)
	}

	// Get project to verify it exists and has images
	projResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || projResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(projResult.Item, &project)

	if project.ImageCount == 0 {
		return errorResponse(400, "Project has no images to zip", headers)
	}

	// Mark as generating by adding a placeholder zip entry
	placeholderZip := ZipFile{
		Key:        "generating",
		Size:       0,
		ImageCount: project.ImageCount,
		CreatedAt:  time.Now().Format(time.RFC3339),
		Status:     "generating",
	}

	zipFilesList, _ := dynamodbattribute.MarshalList([]ZipFile{placeholderZip})
	_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
		UpdateExpression: aws.String("SET ZipFiles = :zips"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":zips": {L: zipFilesList},
		},
	})
	if err != nil {
		fmt.Printf("Error setting zip status: %v\n", err)
	}

	// Invoke the zip Lambda asynchronously
	payload, _ := json.Marshal(map[string]string{"projectId": projectID})

	_, err = lambdaClient.Invoke(&lambdasvc.InvokeInput{
		FunctionName:   aws.String(zipLambdaName),
		InvocationType: aws.String("Event"), // Async invocation
		Payload:        payload,
	})
	if err != nil {
		fmt.Printf("Error invoking zip lambda: %v\n", err)
		return errorResponse(500, "Failed to start zip generation", headers)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"message": "Zip generation started",
	})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleGetZipDownload(projectID string, zipKey string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get project to verify the zip exists
	projResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || projResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(projResult.Item, &project)

	// Find the zip file in the project's zip files list
	var targetZip *ZipFile
	for _, zf := range project.ZipFiles {
		// The zipKey in the request might be URL-encoded, so we need to match the base filename
		if strings.HasSuffix(zf.Key, zipKey) || zf.Key == zipKey {
			targetZip = &zf
			break
		}
	}

	if targetZip == nil || targetZip.Status != "complete" {
		return errorResponse(404, "Zip file not found or not ready", headers)
	}

	// Generate presigned URL for download
	filename := targetZip.Key[strings.LastIndex(targetZip.Key, "/")+1:]
	req, _ := s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket:                     aws.String(bucketName),
		Key:                        aws.String(targetZip.Key),
		ResponseContentDisposition: aws.String(fmt.Sprintf("attachment; filename=\"%s\"", filename)),
	})

	url, err := req.Presign(60 * time.Minute) // 1 hour for large downloads
	if err != nil {
		return errorResponse(500, "Failed to generate download URL", headers)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"url":      url,
		"filename": filename,
		"size":     targetZip.Size,
	})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleGetZipLogs(projectID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get project to verify it exists
	projResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || projResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(projResult.Item, &project)

	// Check if there's a generating zip
	var generatingZip *ZipFile
	for i, zf := range project.ZipFiles {
		if zf.Status == "generating" {
			generatingZip = &project.ZipFiles[i]
			break
		}
	}

	if generatingZip == nil {
		body, _ := json.Marshal(map[string]interface{}{
			"status":  "no_generation",
			"message": "No zip generation in progress",
		})
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Headers:    headers,
			Body:       string(body),
		}, nil
	}

	// Parse the createdAt timestamp
	createdAt, err := time.Parse(time.RFC3339, generatingZip.CreatedAt)
	if err != nil {
		return errorResponse(500, "Failed to parse generation start time", headers)
	}

	// Check if 15 minutes have passed
	elapsed := time.Since(createdAt)
	if elapsed < 15*time.Minute {
		body, _ := json.Marshal(map[string]interface{}{
			"status":      "generating",
			"message":     "Zip generation in progress",
			"elapsedMins": int(elapsed.Minutes()),
		})
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Headers:    headers,
			Body:       string(body),
		}, nil
	}

	// 15 minutes have passed, search CloudWatch logs for errors
	logGroupName := "/aws/lambda/ProjectZipGenerator"
	startTime := createdAt.Add(-1 * time.Minute).UnixMilli()
	endTime := createdAt.Add(20 * time.Minute).UnixMilli()

	// Search for error messages in logs
	filterInput := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:  aws.String(logGroupName),
		StartTime:     aws.Int64(startTime),
		EndTime:       aws.Int64(endTime),
		FilterPattern: aws.String("ERROR"),
		Limit:         aws.Int64(50),
	}

	var errorMessages []string
	filterResult, err := cwLogsClient.FilterLogEvents(filterInput)
	if err != nil {
		fmt.Printf("Failed to query CloudWatch logs: %v\n", err)
		// Still mark as failed even if we can't get logs
		errorMessages = append(errorMessages, "Zip generation timed out. Unable to retrieve error logs.")
	} else {
		for _, event := range filterResult.Events {
			if event.Message != nil {
				errorMessages = append(errorMessages, *event.Message)
			}
		}
	}

	if len(errorMessages) == 0 {
		errorMessages = append(errorMessages, "Zip generation timed out with no error logs found.")
	}

	// Update the project to mark the zip as failed
	var updatedZipFiles []ZipFile
	for _, zf := range project.ZipFiles {
		if zf.Status == "generating" {
			zf.Status = "failed"
		}
		updatedZipFiles = append(updatedZipFiles, zf)
	}

	zipFilesList, _ := dynamodbattribute.MarshalList(updatedZipFiles)
	ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
		UpdateExpression: aws.String("SET ZipFiles = :zips"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":zips": {L: zipFilesList},
		},
	})

	body, _ := json.Marshal(map[string]interface{}{
		"status":        "failed",
		"message":       "Zip generation failed after timeout",
		"errorMessages": errorMessages,
	})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func extractToken(headers map[string]string) string {
	auth := headers["Authorization"]
	if auth == "" {
		auth = headers["authorization"]
	}
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func validateToken(tokenString string) bool {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	return err == nil && token.Valid
}

func errorResponse(statusCode int, message string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	body, _ := json.Marshal(map[string]string{"error": message})
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func main() {
	lambda.Start(handler)
}
