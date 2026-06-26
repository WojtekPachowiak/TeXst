package main

import (
	"context"
	"log"
	"net/http"

	"github.com/kelseyhightower/envconfig"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

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

// const (
// 	streamLatex = "stream:latex"
// 	streamTypst = "stream:typst"
// 	groupLatex  = "workers-latex"
// 	groupTypst  = "workers-typst"
// )

type Environment struct {
	REDIS_ADDR                  string
	STREAM_LATEX                string
	STREAM_TYPST                string
	CONSMUMER_GROUP_LATEX       string
	CONSUMER_GROUP_TYPST        string
	MINIO_BUCKET_NAME           string
	REDIS_PUBSUB_CHANNEL_PREFIX string
}

var env Environment

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "restapi_http_requests_total",
		Help: "Total HTTP requests by method, path and status",
	}, []string{"method", "path", "status"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "restapi_http_request_duration_seconds",
		Help:    "HTTP request latency",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	JobsSubmitted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "restapi_jobs_submitted_total",
		Help: "Render jobs submitted by engine and format",
	}, []string{"engine", "format"})

	// QueueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
	// 	Name: "restapi_queue_depth",
	// 	Help: "Current Redis queue length by format",
	// }, []string{"format"})
)

func main() {
	err := envconfig.Process("", &env)
	if err != nil {
		log.Fatal(err.Error())
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr: env.REDIS_ADDR,
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Redis connection failed:", err)
	}

	minioClient, _, err = shared.NewMinioClient(false)
	if err != nil {
		log.Fatal(err.Error())
	}

	redisClient.XGroupCreateMkStream(ctx, env.STREAM_LATEX, env.CONSMUMER_GROUP_LATEX, "0").Err()
	redisClient.XGroupCreateMkStream(ctx, env.STREAM_TYPST, env.CONSUMER_GROUP_TYPST, "0").Err()

	mux := http.NewServeMux()
	// prometheus metrics server
	mux.Handle("/", http.FileServer(http.Dir("./static")))
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/api/jobs", submitJobHandler)
	mux.HandleFunc("/api/jobs/{id}", getJobInfoHandler)
	mux.HandleFunc("/api/jobs/{id}/stream", streamJobHandler)
	// wrappedMux := mainMiddleware(mux)

	log.Println("API listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
