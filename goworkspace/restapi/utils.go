package main

import (
	"context"
	"fmt"
	"net/http"
)

func getUserFromHeaders(r *http.Request) (string,error) {
	user := r.Header.Get("X-Auth-Request-User")
	if user == ""{
		return "", fmt.Errorf("no authentication user header - is this route behind oauth2-proxy?")
	}
	return user, nil
}

func getJobInfo(ctx context.Context, jobID string) (*JobInfo, error) {
	infoMap, err := redisClient.HGetAll(ctx, fmt.Sprintf("job:%s", jobID)).Result()
	if err != nil || len(infoMap) == 0 {
		return nil, fmt.Errorf("Job not found")
	}

	jobInfo := JobInfo{
		Status:    JobStatus(infoMap["status"]),
		CreatedAt: infoMap["created_at"],
		Engine:    infoMap["engine"],
		Format:    infoMap["format"],
		UpdatedAt: infoMap["updated_at"],
		Error:     infoMap["error"],
		ResultURL: infoMap["result_url"],
	}

	return &jobInfo, nil
}

func getJobStatus(ctx context.Context, jobID string) (string, error) {
	status, err := redisClient.HGet(ctx, fmt.Sprintf("job:%s", jobID), "status").Result()
	if err != nil {
		return "", fmt.Errorf("Job not found")
	}

	return status, nil
}
