package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	sess              *session.Session
	ddbClient         *dynamodb.DynamoDB
	s3Client          *s3.S3
	bucketName        string
	imageTable        string
	usersTable        string
	reviewGroupsTable string
	projectsTable     string
	adminUsername     string
	adminPassword     string
	jwtSecret         = []byte("kill-snap-secret-key-change-in-production")
)

func init() {
	sess = session.Must(session.NewSession())
	ddbClient = dynamodb.New(sess)
	s3Client = s3.New(sess)
	bucketName = os.Getenv("BUCKET_NAME")
	imageTable = os.Getenv("IMAGE_TABLE")
	usersTable = os.Getenv("USERS_TABLE")
	reviewGroupsTable = os.Getenv("REVIEW_GROUPS_TABLE")
	projectsTable = os.Getenv("PROJECTS_TABLE")
	adminUsername = os.Getenv("ADMIN_USERNAME")
	adminPassword = os.Getenv("ADMIN_PASSWORD")

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
	Promoted         bool              `json:"promoted,omitempty"`
	EXIFData         map[string]string `json:"exifData,omitempty"`
	RelatedFiles     []string          `json:"relatedFiles,omitempty"`
	InsertedDateTime string            `json:"insertedDateTime,omitempty"`
	UpdatedDateTime  string            `json:"updatedDateTime,omitempty"`
}

type UpdateImageRequest struct {
	GroupNumber int    `json:"groupNumber,omitempty"`
	ColorCode   string `json:"colorCode,omitempty"`
	Promoted    bool   `json:"promoted,omitempty"`
	Reviewed    string `json:"reviewed,omitempty"`
}

type Project struct {
	ProjectID  string `json:"projectId"`
	Name       string `json:"name"`
	CreatedAt  string `json:"createdAt"`
	ImageCount int    `json:"imageCount"`
}

type CreateProjectRequest struct {
	Name string `json:"name"`
}

type AddToProjectRequest struct {
	All   bool `json:"all,omitempty"`
	Group int  `json:"group,omitempty"`
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

	// Move related files (same base name, different extensions)
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

func getColorName(groupNumber int) string {
	colors := map[int]string{
		0: "none", 1: "red", 2: "blue", 3: "green", 4: "yellow",
		5: "purple", 6: "orange", 7: "pink", 8: "brown",
	}
	if name, ok := colors[groupNumber]; ok {
		return name
	}
	return "none"
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	headers := map[string]string{
		"Content-Type":                "application/json",
		"Access-Control-Allow-Origin": "*",
		"Access-Control-Allow-Headers": "Content-Type,Authorization",
		"Access-Control-Allow-Methods": "GET,POST,PUT,DELETE,OPTIONS",
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
	case strings.HasPrefix(path, "/api/images/") && method == "DELETE":
		imageID := strings.TrimPrefix(path, "/api/images/")
		return handleDeleteImage(imageID, headers)
	// Project routes
	case path == "/api/projects" && method == "GET":
		return handleListProjects(headers)
	case path == "/api/projects" && method == "POST":
		return handleCreateProject(request, headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.HasSuffix(path, "/images") && method == "POST":
		projectID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/projects/"), "/images")
		return handleAddToProject(projectID, request, headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.HasSuffix(path, "/images") && method == "GET":
		projectID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/projects/"), "/images")
		return handleGetProjectImages(projectID, headers)
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
	case "all":
		// Scan all images
		scanResult, scanErr := ddbClient.Scan(&dynamodb.ScanInput{
			TableName: aws.String(imageTable),
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
	var newPaths map[string]string
	var newStatus string

	if updateReq.Reviewed == "true" && img.Reviewed == "false" {
		imageDate := getImageDate(img)
		datePath := buildDatePath(imageDate)

		if updateReq.GroupNumber > 0 {
			// Approved with a group - move to approved/<color>/YYYY/MM/DD
			colorName := getColorName(updateReq.GroupNumber)
			destPrefix := fmt.Sprintf("approved/%s/%s", colorName, datePath)
			newStatus = "approved"

			newPaths, err = moveImageFiles(bucketName, img, destPrefix)
			if err != nil {
				fmt.Printf("Error moving files to approved: %v\n", err)
				return errorResponse(500, fmt.Sprintf("Failed to move files: %v", err), headers)
			}
		} else {
			// Rejected (group 0) - move to rejected/YYYY/MM/DD
			destPrefix := "rejected/" + datePath
			newStatus = "rejected"

			newPaths, err = moveImageFiles(bucketName, img, destPrefix)
			if err != nil {
				fmt.Printf("Error moving files to rejected: %v\n", err)
				return errorResponse(500, fmt.Sprintf("Failed to move files: %v", err), headers)
			}
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

	if updateReq.Reviewed != "" {
		updateExpr += ", Reviewed = :reviewed"
		exprAttrValues[":reviewed"] = &dynamodb.AttributeValue{S: aws.String(updateReq.Reviewed)}
	}

	// Add file path updates if files were moved
	if newPaths != nil {
		updateExpr += ", OriginalFile = :orig, Thumbnail50 = :t50, Thumbnail400 = :t400, #status = :status"
		exprAttrValues[":orig"] = &dynamodb.AttributeValue{S: aws.String(newPaths["original"])}
		exprAttrValues[":t50"] = &dynamodb.AttributeValue{S: aws.String(newPaths["thumbnail50"])}
		exprAttrValues[":t400"] = &dynamodb.AttributeValue{S: aws.String(newPaths["thumbnail400"])}
		exprAttrValues[":status"] = &dynamodb.AttributeValue{S: aws.String(newStatus)}
		exprAttrNames["#status"] = aws.String("Status")
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

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       `{"success": true}`,
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

	project := Project{
		ProjectID:  uuid.New().String(),
		Name:       req.Name,
		CreatedAt:  time.Now().Format(time.RFC3339),
		ImageCount: 0,
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

	body, _ := json.Marshal(project)
	return events.APIGatewayProxyResponse{
		StatusCode: 201,
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

	// Move each image to project folder
	movedCount := 0
	for _, item := range result.Items {
		var img ImageResponse
		dynamodbattribute.UnmarshalMap(item, &img)

		imageDate := getImageDate(img)
		datePath := buildDatePath(imageDate)
		destPrefix := fmt.Sprintf("projects/%s/%s", project.Name, datePath)

		newPaths, err := moveImageFiles(bucketName, img, destPrefix)
		if err != nil {
			fmt.Printf("Failed to move image %s: %v\n", img.ImageGUID, err)
			continue
		}

		// Update image record
		_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(img.ImageGUID)},
			},
			UpdateExpression: aws.String("SET OriginalFile = :orig, Thumbnail50 = :t50, Thumbnail400 = :t400, #status = :status, ProjectID = :proj, UpdatedDateTime = :updated"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":orig":    {S: aws.String(newPaths["original"])},
				":t50":     {S: aws.String(newPaths["thumbnail50"])},
				":t400":    {S: aws.String(newPaths["thumbnail400"])},
				":status":  {S: aws.String("project")},
				":proj":    {S: aws.String(projectID)},
				":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
			},
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
