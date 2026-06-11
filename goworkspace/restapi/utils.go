package main

import (
	"context"
	"time"
	"fmt"
)

func getPresignedURL(ctx context.Context, objectName string) (string, error) {
	bucketName := env.MINIO_BUCKET_NAME

	expires := time.Duration(15) * time.Minute

	presignedURL, err := minioClient.PresignedGetObject(ctx, bucketName, objectName, expires, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generated presigned URL: %w", err)
	}

	return presignedURL.String(), nil
}

func getJobStatusStruct(ctx context.Context, jobID string) (*JobStatus, error) {
	statusMap, err := redisClient.HGetAll(ctx, fmt.Sprintf("job:%s", jobID)).Result()
	if err != nil || len(statusMap) == 0 {
		return nil, fmt.Errorf("Job not found")

	}

	jobStatus := JobStatus{
		Status:    statusMap["status"],
		CreatedAt: statusMap["created_at"],
		UpdatedAt: statusMap["updated_at"],
		Error:     statusMap["error"],
		ResultURL: statusMap["result_url"],
	}

	return &jobStatus, nil
}

func getJobStatusField(ctx context.Context, jobID string) (string, error) {
	status, err := redisClient.HGet(ctx, fmt.Sprintf("job:%s", jobID), "status").Result()
	if err != nil {
		return "", fmt.Errorf("Job not found")
	}

	return status, nil
}
