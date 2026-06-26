package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"shared"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func submitJobHandler(w http.ResponseWriter, r *http.Request) {
	// if r.Method != http.MethodPost {
	// 	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	// 	return
	// }

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
		slog.Error("failed to enque job", "error", err, "job_id", jobID)
		http.Error(w, "Failed to enqueue job", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339) //2006-01-02T15:04:05Z07:00
	status := JobInfo{
		Status:    JobStatusQueued,
		CreatedAt: now,
		UpdatedAt: now,
		Engine:    engine,
		Format:    format,
		ResultURL: "",
		Error:     "",
	}

	err = redisClient.HSet(ctx, fmt.Sprintf("job:%s", jobID),
		status.ToMap()).Err()
	if err != nil {
		log.Printf("Warning: failed to save initial status: %v", err)
	}

	JobsSubmitted.WithLabelValues(job.Engine, job.Format).Inc()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"job_id": jobID,
		"status": string(JobStatusQueued),
	})

}

func getJobInfoHandler(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	// jobID := filepath.Base(r.URL.Path)
	if jobID == "" {
		http.Error(w, "Couldn't extract the job's ID", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	resp, err := getJobInfo(ctx, jobID)
	if err != nil {
		http.Error(w, "Could not get job info", http.StatusNotFound)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func streamJobHandler(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")

	// jobID := filepath.Base(r.URL.Path)
	if jobID == "" {
		http.Error(w, "Missing job ID", http.StatusBadRequest)
		return
	}

	ctx := context.Background()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") //nginx hint
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sub := redisClient.Subscribe(ctx, env.REDIS_PUBSUB_CHANNEL_PREFIX+jobID)
	defer sub.Close()
	ch := sub.Channel()

	if jobInfo, err := getJobInfo(ctx, jobID); err == nil {
		d, _ := json.Marshal(jobInfo.ToMap())
		// TODO: err handle
		fmt.Fprintf(w, "event: info\ndata: %s\n\n", d)
		flusher.Flush()

		if jobInfo.Status == JobStatusCompleted || jobInfo.Status == JobStatusFailed {
			return //nothing more to stream
		}
	}

	// ping every 15 seconds
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	timeout := time.NewTimer(5 * time.Minute)
	defer timeout.Stop()

	for {
		select {
		case <-timeout.C:
			pubSubMsg := JobPubSubMsg{
				ID:        jobID,
				Status:    JobStatusFailed,
				Error:     "timed out waiting for worker",
				ResultURL: "",
			}
			d, _ := json.Marshal(pubSubMsg.ToMap())
			fmt.Fprintf(w, "event: info\ndata: %s\n\n", d)
			flusher.Flush()
			return

		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: info\ndata: %s\n\n", msg.Payload)
			flusher.Flush()

			var pubSubMsg JobPubSubMsg
			err := json.Unmarshal([]byte(msg.Payload), &pubSubMsg)
			if err !=nil {
				http.Error(w, "error parsing pubsub message", http.StatusInternalServerError)
				return
			}

			if  (pubSubMsg.Status == JobStatusCompleted || pubSubMsg.Status == shared.JobStatusFailed) {
				return
			}

		case <-keepalive.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()

		case <-ctx.Done():
			return //client disconnected
		}
	}

	// status, err := getJobStatus(ctx, jobID)
	// if err != nil {
	// 	http.Error(w, "Could not get job status", http.StatusInternalServerError)
	// 	return
	// }
	// if status == JobStatusFailed {
	// 	http.Error(w, fmt.Sprintf("Job not completed (status: %s)", status), http.StatusInternalServerError)
	// 	return
	// } else if status != JobStatusCompleted {
	// 	http.Error(w, fmt.Sprintf("Job not completed (status: %s)", status), http.StatusAccepted)
	// 	return
	// }

	// jobInfo, err := getJobInfo(ctx, jobID)
	// if err != nil {
	// 	http.Error(w, "Could not get job info", http.StatusInternalServerError)
	// 	return
	// }

	// if jobInfo.ResultURL == "" {
	// 	http.Error(w, "Could not locate the result's location", http.StatusInternalServerError)
	// 	return
	// }

	// objectName := jobInfo.ResultURL
	// presignedURL, err := getPresignedURL(ctx, objectName)
	// if err != nil {
	// 	http.Error(w, "Could not generate presigned URL", http.StatusInternalServerError)
	// 	log.Printf("Error: %s", err)
	// 	return
	// }

	// resp := map[string]string{"url": presignedURL}
	// w.Header().Set("Content-Type", "application/json")
	// json.NewEncoder(w).Encode(resp)

}

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

// w.Header().Set("Content-Type", "application/pdf")
// w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s.%s", jobID, fileExt))
// http.ServeFile(w, r, filePath)
