package restapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func renderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	engine := r.FormValue("engine")
	source := r.FormValue("source")
	format := r.FormValue("format")
	if engine != "latex" && engine != "typst" {
		http.Error(w, "Invalid engine", http.StatusBadRequest)
		return
	}
	if source == "" {
		http.Error(w, "Source cannot be empty", http.StatusBadRequest)
		return
	}
	if format != "pdf" && format != "png" {
		http.Error(w, "Invalid format", http.StatusBadRequest)
		return
	}

	jobID := uuid.New().String()
	job := Job{
		ID:     jobID,
		Engine: engine,
		Source: source,
		Format: format,
	}

	jobData, err := json.Marshal(job)
	if err != nil {
		http.Error(w, "Failed to serialize job", http.StatusInternalServerError)
		return
	}

	var targetStream string

	switch engine {
	case "latex":
		targetStream = env.STREAM_LATEX
	case "typst":
		targetStream = env.STREAM_TYPST
	}

	ctx := context.Background()

	// Add job to Redis Stream
	_, err = redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: targetStream,
		Values: map[string]any{
			"job_id":   jobID,
			"job_data": string(jobData),
		},
	}).Result()
	if err != nil {
		http.Error(w, "Failed to enqueue job", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339) //2006-01-02T15:04:05Z07:00
	status := map[string]any{
		"status":     "queued",
		"created_at": now,
		"updated_at": now,
		"engine":     engine,
		"format":     format,
		"result_url": "",
		"error":      "",
	}
	err = redisClient.HSet(ctx, fmt.Sprintf("job:%s", jobID), status).Err()
	if err != nil {
		log.Printf("Warning: failed to save initial status: %v", err)
	}

}

func statusHandler(w http.ResponseWriter, r *http.Request) {

	jobID := filepath.Base(r.URL.Path)
	if jobID == "" {
		http.Error(w, "Missing job ID", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	resp, err := getJobStatusStruct(ctx, jobID)
	if err != nil {
		http.Error(w, "Could not get job status", http.StatusNotFound)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func resultHandler(w http.ResponseWriter, r *http.Request) {

	jobID := filepath.Base(r.URL.Path)
	if jobID == "" {
		http.Error(w, "Missing job ID", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	status, err := getJobStatusField(ctx, jobID)
	if status != "completed" {
		http.Error(w, fmt.Sprintf("Job not completed (status: %s)", status), http.StatusAccepted)
		return
	}

	jobStatus, err := getJobStatusStruct(ctx, jobID)
	if err != nil {
		http.Error(w, "Could not get job status", http.StatusInternalServerError)
	}

	if jobStatus.ResultURL == "" {
		http.Error(w, "Could not locate the result's location", http.StatusInternalServerError)
	}

	objectName := jobStatus.ResultURL
	presignedURL, err := getPresignedURL(ctx, objectName)

	// files, err := filepath.Glob(filepath.Join("rendered", jobID, "output") + ".*" )
	// if err != nil {
	// 	http.Error(w,"[Internal Server Error] Searching for rendered result failed.",http.StatusInternalServerError)
	// 	return
	// }
	// if len(files) != 0 {
	// 	http.Error(w,"[Internal Server Error] The file count should be 1.",http.StatusInternalServerError)
	// 	return
	// }

	// filePath := files[0]
	// fileExt := filepath.Ext(filePath)
	// log.Printf("File path is %s", filePath)
	// if _, err := os.Stat(filePath); os.IsNotExist(err){
	// 	http.Error(w, "Result file not found", http.StatusNotFound)
	// 	return
	// }

	resp := map[string]string{"url": presignedURL}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

	// w.Header().Set("Content-Type", "application/pdf")
	// w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s.%s", jobID, fileExt))
	// http.ServeFile(w, r, filePath)
}
