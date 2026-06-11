package restapi

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/kelseyhightower/envconfig"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"

	"tex-typst-rendering-cluster/storage"
)

type Job struct {
	ID     string `json:"id"`
	Engine string `json:"engine"` // latex or typst
	Source string `json:"source"` // source file text
	Format string `json:"format"` // "pdf" or "png"
}

type JobStatus struct {
	Status    string `json:"status"` //queued, processing, completed, failed
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Error     string `json:"error,omitempty"`
	ResultURL string `json:"result_url,omitempty"`
}

var redisClient *redis.Client
var minioClient *minio.Client

// const (
// 	streamLatex = "stream:latex"
// 	streamTypst = "stream:typst"
// 	groupLatex  = "workers-latex"
// 	groupTypst  = "workers-typst"
// )

type Environment struct {
	REDIS_ADDR            string
	STREAM_LATEX          string
	STREAM_TYPST          string
	CONSMUMER_GROUP_LATEX string
	CONSUMER_GROUP_TYPST  string
	MINIO_BUCKET_NAME     string
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

	minioClient, err = storage.InitMinio()
	if err != nil {
		log.Fatal(err.Error())
	}

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Redis connection failed:", err)
	}

	redisClient.XGroupCreateMkStream(ctx, env.STREAM_LATEX, env.CONSMUMER_GROUP_LATEX, "0").Err()
	redisClient.XGroupCreateMkStream(ctx, env.STREAM_TYPST, env.CONSUMER_GROUP_TYPST, "0").Err()

	os.MkdirAll("rendered", 0755)

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/render", renderHandler)
	http.HandleFunc("/status/", statusHandler)
	http.HandleFunc("/result/", resultHandler)
	http.Handle("/rendered/", http.StripPrefix("/rendered/", http.FileServer(http.Dir("./rendered"))))

	log.Println("API listening on http:/localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
