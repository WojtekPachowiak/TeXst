package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"

	"storage"
)

type Job struct {
	ID     string `json:"id"`
	Engine string `json:"engine"`
	Source string `json:"source"`
	Format string `json:"format"`
}

var redisClient *redis.Client
var minioClient *minio.Client

// const streamName = "rendering-jobs"
// const consumerGroup = "workers"
// const consumerName = "worker-1" // each worker should have a unique name

type Environment struct {
	REDIS_ADDR        string
	CONSUMER_GROUP    string
	CONSUMER_NAME     string
	STREAM_NAME       string
	WORKER_ENGINE     string
	MINIO_BUCKET_NAME string
}

var env Environment

func main() {
	err := envconfig.Process("", &env)
	if err != nil {
		log.Fatal(err.Error())
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr: env.REDIS_ADDR,
	})
	
	minioClient, _, err = storage.InitMinio()
	if err != nil {
		log.Fatal(err.Error())
	}

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Redis connection failed:", err)
	}

	if err := redisClient.XGroupCreateMkStream(ctx, env.STREAM_NAME, env.CONSUMER_GROUP, "0").Err(); err != nil {
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			log.Fatal("Failed to create consumer group:", err)
		}
	}

	os.MkdirAll("rendered", 0755)

	log.Println("Worker started, waiting for jobs...")

	for {
		streams, err := redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    env.CONSUMER_GROUP,
			Consumer: env.CONSUMER_NAME,
			Streams:  []string{env.STREAM_NAME, ">"},
			Count:    1,
			Block:    5 * time.Second,
		}).Result()
		if err != nil {
			if err == redis.Nil {
				continue
			}
			log.Printf("XReadGroup error: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		//TODO: czy dobrze, że funkcje tworzą własny ctx?

		//TODO: czy potrzeba XAUTOCLAIM?

		for _, stream := range streams {
			for _, message := range stream.Messages {
				jobID := message.Values["job_id"].(string)
				jobDataStr := message.Values["job_data"].(string)

				processMessage(jobID, jobDataStr)
				acknowledgeMessage(message.ID)
			}
		}
	}
}

func processMessage(jobID, jobDataStr string) error {
	var job Job
	if err := json.Unmarshal([]byte(jobDataStr), &job); err != nil {
		log.Printf("Failed to unmarshal job %s: %v", jobID, err)
		markJobFailed(jobID, "Invalid job data")
		return err
	}

	if job.Engine != env.WORKER_ENGINE {
		err := fmt.Errorf("Worker received wrong job (engines do not match).")
		log.Print("%s", err)
		markJobFailed(jobID, "Invalid job engine")
		return err
	}

	updateJobStatus(jobID, "processing", "", "")

	outputPath, err := renderDocument(job)
	if err != nil {
		log.Printf("Job %s failed: %v", jobID, err)
		markJobFailed(jobID, err.Error())
		return err
	} else {
		log.Printf("Job %s processed and awaits upload to storage", jobID)
		updateJobStatus(jobID, "uploading", "", "")
	}

	resultURL, err := uploadToMinIO(jobID, outputPath)
	if err != nil {
		log.Printf("Job %s could not be upload to storage: %v", jobID, err)
		markJobFailed(jobID, err.Error())
		return err
	} else {
		updateJobStatus(jobID, "completed", "", resultURL)
		log.Printf("Job %s processed and uploaded to storage", jobID)
	}

	return nil
}

func renderDocument(job Job) (string, error) {
	jobDir := filepath.Join("rendered", job.ID)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create job directory: %w", err)
	}

	var inputExt string
	switch job.Engine {
	case "latex":
		inputExt = "tex"
	case "typst":
		inputExt = "typ"
	default:
		return "", fmt.Errorf("unsupported engine: %s", job.Engine)
	}

	inputPath := filepath.Join(jobDir, "input."+inputExt)
	if err := os.WriteFile(inputPath, []byte(job.Source), 0644); err != nil {
		return "", fmt.Errorf("failed to write input file: %w", err)
	}

	pdfPath := filepath.Join(jobDir, "output.pdf")

	var cmd *exec.Cmd
	switch job.Engine {
	case "latex":
		// cmd = exec.Command("pdflatex", inputPath)
		cmd = exec.Command("pandoc", inputPath, "-o", pdfPath, "--pdf-engine=pdflatex")
	case "typst":
		cmd = exec.Command("pandoc", inputPath, "-o", pdfPath, "--pdf-engine=typst")
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pandoc execution failed: %s\n%s", err, out)
	}

	var outputPath string
	switch job.Format {
	case "pdf":
		outputPath = pdfPath
	case "png":
		pngPath := strings.Replace(pdfPath, ".pdf", ".png", -1)

		cmd = exec.Command("magick", "-density", "150", pdfPath, "-quality", "90", pngPath)
		out, err = cmd.CombinedOutput()

		if err != nil {
			return "", fmt.Errorf("failed to convert pdf to png: %s\n%s", err, out)
		}
		outputPath = pngPath
	default:
		return "", fmt.Errorf("unsupported format: %s", job.Format)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("output file not generated")
	}

	return outputPath, nil
}

func updateJobStatus(jobID, status, errMsg, resultURL string) {
	ctx := context.Background()
	updates := map[string]any{
		"status":     status,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
		"error":      errMsg,
		"result_url": resultURL,
	}
	if err := redisClient.HSet(ctx, fmt.Sprintf("job:%s", jobID), updates).Err(); err != nil {
		log.Printf("Failed to update status for job %s: %v", jobID, err)
	}
}

func markJobFailed(jobID, errMsg string) {
	updateJobStatus(jobID, "failed", errMsg, "")
}

func acknowledgeMessage(messageID string) {
	ctx := context.Background()
	if err := redisClient.XAck(ctx, env.STREAM_NAME, env.CONSUMER_GROUP, messageID).Err(); err != nil {
		log.Printf("Failed to acknowledge message %s: %v", messageID, err)
	}
}

func uploadToMinIO(jobID, outputPath string) (string, error) {
	ctx := context.Background()

	bucketName := env.MINIO_BUCKET_NAME
	// exists, err := minioClient.BucketExists(ctx, bucketName)
	// if err != nil {
	// 	return "", fmt.Errorf("failed to check bucket: %w", err)
	// }
	// if !exists {
	// 	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	// 	if err != nil {
	// 		return "", fmt.Errorf("failed to create bucket: %w", err)
	// 	}
	// 	log.Printf("Bucket '%s' created", bucketName)
	// }

	ext := filepath.Ext(outputPath)
	var contentType string
	switch ext {
	case ".png":
		contentType = "image/png"
	case ".pdf":
		contentType = "application/pdf"
	default:
		return "", fmt.Errorf("the file to be uploaded has unsupported format: %s", ext)
	}
	objectName := fmt.Sprintf("%s/output"+ext, jobID)

	// Upload from the local path 'filePath' to the bucket
	_, err := minioClient.FPutObject(ctx, bucketName, objectName, outputPath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	log.Printf("Successfully uploaded %s to bucket %s", objectName, bucketName)
	return objectName, nil
}
