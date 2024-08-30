package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func loadEnv() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file:", err)
		os.Exit(1)
	}
}

var db *gorm.DB
var s3Client *s3.S3
var s3bucketName = "dropbox-test-123"
var s3Region = "ap-southeast-1"

type UploadMetadataStatus string

const (
	UploadMetadataStatusInit      UploadMetadataStatus = "init"
	UploadMetadataStatusCompleted UploadMetadataStatus = "completed"
)

// UploadMetadata model
type UploadMetadata struct {
	UploadID     uuid.UUID            `gorm:"type:uuid;primaryKey" json:"upload_id"`
	Namespace    string               `json:"namespace"`     // user_id
	RelativePath string               `json:"relative_path"` // /folder1/folder2
	Version      int                  `json:"version"`       // 1
	FileName     string               `json:"file_name"`     // file.txt
	FileSize     int64                `json:"file_size"`     // 2048 (bytes)
	ChunksTotal  int                  `json:"chunks_total"`  // 2
	Status       UploadMetadataStatus `json:"status"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
	Chunks       []UploadChunk        `gorm:"foreignKey:UploadID" json:"chunks"`
}

type UploadChunkStatus string

const (
	UploadChunkStatusInit      UploadChunkStatus = "init"
	UploadChunkStatusCompleted UploadChunkStatus = "completed"
)

// UploadChunk model
type UploadChunk struct {
	ChunkID    uint              `gorm:"primaryKey" json:"chunk_id"`
	UploadID   uuid.UUID         `gorm:"type:uuid;index" json:"upload_id"`
	ChunkIndex int               `json:"chunk_index"`
	SignedURL  string            `json:"signed_url"`
	S3URL      string            `json:"s3_url"`
	Status     UploadChunkStatus `json:"status"`
	ChunkHash  string            `json:"chunk_hash"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

func main() {
	loadEnv()
	initDB()
	initS3()

	router := gin.Default()
	// Step 1
	router.POST("/start-upload", startUploadHandler)
	// Step 2
	router.GET("/get-signed-url", getSignedURLHandler)
	// Step 3
	router.POST("/confirm-chunk", confirmChunkHandler)

	fmt.Println("Server started at :8080")
	router.Run(":8080")
}

func initDB() {
	dsn := "user=postgres dbname=postgres sslmode=disable password=123456 host=localhost port=5432"
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("Error connecting to database:", err)
		os.Exit(1)
	}

	// Auto migrate the UploadMetadata and UploadChunk models
	db.AutoMigrate(&UploadMetadata{}, &UploadChunk{})
}

func initS3() {
	accessKey := os.Getenv("S3_ACCESS_KEY")
	secretAccessKey := os.Getenv("S3_SECRET_ACCESS_KEY")

	sess := session.Must(session.NewSession(&aws.Config{
		Region:           aws.String(s3Region),
		Credentials:      credentials.NewStaticCredentials(accessKey, secretAccessKey, ""),
		S3ForcePathStyle: aws.Bool(true),
	}))
	s3Client = s3.New(sess)
	// Test the connection
	_, err := s3Client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		fmt.Println("Error connecting to S3:", err)
		os.Exit(1)
	}
}

// This only handle new upload
// TODO: handle versioning upload (overwrite existing file)
func startUploadHandler(c *gin.Context) {
	fileName := c.PostForm("fileName")
	fileSize, _ := strconv.ParseInt(c.PostForm("fileSize"), 10, 64)
	chunkHashes := c.PostFormArray("chunkHashes[]")
	version := 1

	uploadID := uuid.New()
	metadata := UploadMetadata{
		UploadID:  uploadID,
		FileName:  fileName,
		FileSize:  fileSize,
		Status:    UploadMetadataStatusInit,
		Version:   version,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save upload metadata to the database
	if err := db.Create(&metadata).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload metadata"})
		return
	}

	chunksToUpload := []UploadChunk{}

	for i, chunkHash := range chunkHashes {
		chunk := UploadChunk{
			UploadID:   uploadID,
			ChunkIndex: i,
			Status:     UploadChunkStatusInit,
			ChunkHash:  chunkHash,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		err := db.Create(&chunk).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload chunk"})
			return
		}
		chunksToUpload = append(chunksToUpload, chunk)
	}

	c.JSON(http.StatusOK, gin.H{"uploadID": uploadID, "chunks": chunksToUpload})
}

func getSignedURLHandler(c *gin.Context) {
	uploadID := c.Query("uploadID")
	chunkHash := c.Query("chunkHash")

	// Generate signed URL for the chunk
	req, _ := s3Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(s3bucketName),
		Key:    aws.String(chunkHash),
	})
	signedURL, err := req.Presign(15 * time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate signed URL"})
		return
	}

	// Update chunk with signed URL
	if err := db.Model(&UploadChunk{}).
		Where("upload_id = ? AND chunk_hash = ?", uploadID, chunkHash).
		Updates(map[string]interface{}{"signed_url": signedURL, "updated_at": time.Now()}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update chunk URL"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"signed_url": signedURL})
}

func confirmChunkHandler(c *gin.Context) {
	uploadID := c.PostForm("uploadID")
	chunkHash := c.PostForm("chunkHash")

	// Update the chunk status to 'uploaded'
	if err := db.Model(&UploadChunk{}).
		Where("upload_id = ? AND chunk_hash = ?", uploadID, chunkHash).
		Updates(map[string]interface{}{"status": UploadChunkStatusCompleted, "updated_at": time.Now()}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update chunk status"})
		return
	}

	// Check if all chunks are uploaded
	var completeCount int64 = 0
	err := db.Model(&UploadChunk{}).Where("upload_id = ? AND status = ?", uploadID, UploadChunkStatusCompleted).Count(&completeCount).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count completed chunks"})
		return
	}

	var allChunksCount int64 = 0
	err = db.Model(&UploadChunk{}).Where("upload_id = ?", uploadID).Count(&allChunksCount).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count all chunks"})
		return
	}

	if completeCount == allChunksCount {
		// Update the upload status to 'completed'
		if err := db.Model(&UploadMetadata{}).
			Where("upload_id = ?", uploadID).
			Updates(map[string]interface{}{"status": UploadMetadataStatusCompleted, "updated_at": time.Now()}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update upload status"})
			return
		}
	}

	c.String(http.StatusOK, "Chunk confirmed")
}
