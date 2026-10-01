package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/minio/minio-go/v7"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	"shared"
)

type Job = shared.Job
type JobInfo = shared.JobInfo
type JobUpdate = shared.JobUpdate
type JobPubSubMsg = shared.JobPubSubMsg
type JobStatus = shared.JobStatus

const (
	JobStatusCompleted  = shared.JobStatusCompleted
	JobStatusFailed     = shared.JobStatusFailed
	JobStatusProcessing = shared.JobStatusProcessing
	JobStatusUploading  = shared.JobStatusUploading
	JobStatusQueued     = shared.JobStatusQueued
)

var redisClient *redis.Client
var minioClient *minio.Client
var minioPresigner *minio.Client

// const streamName = "rendering-jobs"
// const consumerGroup = "workers"
// const consumerName = "worker-1" // each worker should have a unique name

type Environment struct {
	REDIS_ADDR                  string
	WORKER_CONSUMER_GROUP              string
	WORKER_CONSUMER_NAME               string
	WORKER_STREAM_NAME                 string
	WORKER_ENGINE               string
	MINIO_BUCKET_NAME           string
	REDIS_PUBSUB_CHANNEL_PREFIX string
}

var env Environment

var (
	// <!-- JobsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
	// 	Name: "worker_jobs_processed_total",
	// 	Help: "Jobs processed by format and outcome",
	// }, []string{"format", "status"}) // status: success|error|timeout -->

	RenderDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "worker_render_duration_seconds",
		Help:    "Time spent invoking pandoc/typst/latex",
		Buckets: []float64{0.5, 1, 2, 5, 10, 20, 30, 60, 120},
	}, []string{"engine", "format", "status"})

	UploadDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "worker_uploadtostorage_duration_seconds",
		Help:    "Time spent uploading to storage (minio)",
		Buckets: []float64{0.5, 1, 2, 5, 10, 20, 30, 60, 120},
	}, []string{"engine", "format", "status"})

	// <!-- QueueWait = promauto.NewHistogramVec(prometheus.HistogramOpts{
	// 	Name:    "worker_queue_wait_seconds",
	// 	Help:    "Time between enqueue and dequeue",
	// 	Buckets: prometheus.DefBuckets,
	// }, []string{"format"}) -->
)

func main() {
	err := envconfig.Process("", &env)
	if err != nil {
		log.Fatal(err.Error())
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr: env.REDIS_ADDR,
	})

	minioClient, minioPresigner, err = shared.NewMinioClient(true)
	if err != nil {
		log.Fatal(err.Error())
	}

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Redis connection failed:", err)
	}

	if err := redisClient.XGroupCreateMkStream(ctx, env.WORKER_STREAM_NAME, env.WORKER_CONSUMER_GROUP, "0").Err(); err != nil {
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			log.Fatal("Failed to create consumer group:", err)
		}
	}

	// start prometheus metrics server
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":9100", nil)
	}()

	os.MkdirAll("rendered", 0755)

	log.Println("Worker started, waiting for jobs...")

	for {
		streams, err := redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    env.WORKER_CONSUMER_GROUP,
			Consumer: env.WORKER_CONSUMER_NAME,
			Streams:  []string{env.WORKER_STREAM_NAME, ">"},
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
		log.Printf("%s", err)
		markJobFailed(jobID, "Invalid job engine")
		return err
	}

	updateJobInfo(jobID, JobStatusProcessing, "", "")

	start := time.Now()
	outputPath, err := renderDocument(job)
	duration := time.Since(start).Seconds()
	if err != nil {
		log.Printf("Job %s failed: %v", jobID, err)
		markJobFailed(jobID, err.Error())
		RenderDuration.WithLabelValues(job.Engine, job.Format, "failure").Observe(duration)
		return err
	} else {
		log.Printf("Job %s processed and awaits upload to storage", jobID)
		RenderDuration.WithLabelValues(job.Engine, job.Format, "success").Observe(duration)
		updateJobInfo(jobID, JobStatusUploading, "", "")
	}

	start = time.Now()
	objectName, err := uploadToMinIO(jobID, outputPath)
	duration = time.Since(start).Seconds()
	if err != nil {
		log.Printf("Job %s could not be uploaded to storage: %v", jobID, err)
		markJobFailed(jobID, err.Error())
		UploadDuration.WithLabelValues(job.Engine, job.Format, "failure").Observe(duration)
		return err
	} else {
		log.Printf("Job %s uploaded to storage", jobID)
		UploadDuration.WithLabelValues(job.Engine, job.Format, "success").Observe(duration)
	}

	presignedURL, err := getPresignedURL(objectName)
	if err != nil {
		log.Printf("Job %s: failed to generate presignedURL: %v", jobID, err)
		markJobFailed(jobID, err.Error())
		return err
	} else {
		log.Printf("Job %s: generated presignedURL successfully", jobID)
	}

	updateJobInfo(jobID, JobStatusCompleted, "", presignedURL)

	return nil
}

func getPresignedURL(objectName string) (string, error) {
	bucketName := env.MINIO_BUCKET_NAME

	expires := time.Duration(15) * time.Minute

	ctx := context.Background()

	presignedURL, err := minioPresigner.PresignedGetObject(ctx, bucketName, objectName, expires, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generated presigned URL: %w", err)
	}

	return presignedURL.String(), nil
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

func updateJobInfo(jobID string, status JobStatus, errMsg, resultURL string) {
	ctx := context.Background()

	update := JobUpdate{
		Status:    status,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Error:     errMsg,
		ResultURL: resultURL,
	}
	
	if err := redisClient.HSet(ctx, fmt.Sprintf("job:%s", jobID), update.ToMap()).Err(); err != nil {
		log.Printf("Failed to update status for job %s: %v", jobID, err)
	}

	// Publish message to Redis Pub/Sub for restapi
	msg := JobPubSubMsg{
		ID:        jobID,
		Status:    status,
		Error:     errMsg,
		ResultURL: resultURL,
	}
	// msg := map[string]any{
	// 	"job_id":     jobID,
	// 	"status":     status,
	// 	"error":      errMsg,
	// 	"result_url": resultURL,
	// }
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal job status message for job %s: %v", jobID, err)
		return
	}
	redisClient.Publish(ctx, fmt.Sprintf("%s%s", env.REDIS_PUBSUB_CHANNEL_PREFIX, jobID), msgBytes)
}

func markJobFailed(jobID, errMsg string) {
	updateJobInfo(jobID, JobStatusFailed, errMsg, "")
}

func acknowledgeMessage(messageID string) {
	ctx := context.Background()
	if err := redisClient.XAck(ctx, env.WORKER_STREAM_NAME, env.WORKER_CONSUMER_GROUP, messageID).Err(); err != nil {
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
