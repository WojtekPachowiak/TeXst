package worker

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
	"github.com/redis/go-redis/v9"
)

type Job struct {
	ID     string `json:"id"`
	Engine string `json:"engine"`
	Source string `json:"source"`
	Format string `json:"format"`
}

var redisClient *redis.Client

const streamName = "rendering-jobs"
const consumerGroup = "workers"
const consumerName = "worker-1" // each worker should have a unique name

type Environment struct {
	RedisAddr     string
	ConsumerGroup string
	ConsumerName  string
	StreamName    string
	Engine        string
}

var env Environment

func main() {
	err := envconfig.Process("", &env)
	if err != nil {
		log.Fatal(err.Error())
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr: env.RedisAddr,
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Redis connection failed:", err)
	}

	if err := redisClient.XGroupCreateMkStream(ctx, env.StreamName, env.ConsumerGroup, "0").Err(); err != nil {
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			log.Fatal("Failed to create consumer group:", err)
		}
	}

	os.MkdirAll("rendered", 0755)

	log.Println("Worker started, waiting for jobs...")

	for {
		streams, err := redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    env.ConsumerGroup,
			Consumer: env.ConsumerName,
			Streams:  []string{env.StreamName, ">"},
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

		for _, stream := range streams {
			for _, message := range stream.Messages {
				jobID := message.Values["job_id"].(string)
				jobDataStr := message.Values["job_data"].(string)

				var job Job
				if err := json.Unmarshal([]byte(jobDataStr), &job); err != nil {
					log.Printf("Failed to unmarshal job %s: %v", jobID, err)
					markJobFailed(jobID, "Invalid job data")
					acknowledgeMessage(message.ID)
					continue
				}

				if job.Engine != env.Engine {
					log.Print("Worker received wrong job (engines do not match).")
					markJobFailed(jobID, "Invalid job engine")
					acknowledgeMessage(message.ID)
					continue
				}

				updateJobStatus(jobID, "processing", "", "")

				err := processJob(job)
				if err != nil {
					log.Printf("Job %s failed: %v", jobID, err)
					markJobFailed(jobID, err.Error())
				} else {
					resultURL := fmt.Sprintf("/rendered/%s/output.%s", jobID, job.Format)
					updateJobStatus(jobID, "completed", "", resultURL)
					log.Printf("Job %s completed successfully", jobID)
				}
			}
		}
	}
}

func processJob(job Job) error {
	jobDir := filepath.Join("rendered", job.ID)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return fmt.Errorf("failed to create job directory: %w", err)
	}

	var inputExt string
	switch job.Engine {
	case "latex":
		inputExt = "tex"
	case "typst":
		inputExt = "typ"
	default:
		return fmt.Errorf("unsupported engine: %s", job.Engine)
	}

	inputPath := filepath.Join(jobDir, "input."+inputExt)
	if err := os.WriteFile(inputPath, []byte(job.Source), 0644); err != nil {
		return fmt.Errorf("failed to write input file: %w", err)
	}

    outputPath := filepath.Join(jobDir, "output."+job.Format)

	var cmd *exec.Cmd
	switch job.Engine {
	case "latex":
		cmd = exec.Command("pandoc", inputPath, "-o", outputPath, "--pdf-engine=xelatex")
	case "typst":
		cmd = exec.Command("pandoc", inputPath, "-o", outputPath, "--pdf-engine=typst")
	}

    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("pandoc execution failed: %s\n%s", err, output)
    }

    if _, err := os.Stat(outputPath); os.IsNotExist(err) {
        return fmt.Errorf("output file not generated")
    }

    return nil
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
	if err := redisClient.XAck(ctx, env.StreamName, env.ConsumerGroup, messageID).Err(); err != nil {
		log.Printf("Failed to acknowledge message %s: %v", messageID, err)
	}
}
