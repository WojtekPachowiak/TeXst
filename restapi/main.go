package restapi

import (
	"context"
	"net/http"
	"os"
	"log"
	"github.com/redis/go-redis/v9"
)

type Job struct {
	ID 	string	`json:"id"`
	Engine 	string	`json:"engine"` // latex or typst
	Source 	string	`json:"source"` // source file text
	Format 	string	`json:"format"` // "pdf" or "png"
}

type JobStatus struct {
	Status 	string	`json:"status"` //queued, processing, completed, failed
	CreatedAt 	string	`json:"created_at"`
	UpdatedAt 	string	`json:"updated_at"`
	Error 	string	`json:"error,omitempty"`
	ResultURL 	string	`json:"result_url,omitempty"`
}

var redisClient *redis.Client
const (
	streamLatex = "stream:latex"
	streamTypst = "stream:typst"
	groupLatex = "workers-latex"
	groupTypst = "workers-typst"
)

func main(){
	
	redisAddr, nonempty := os.LookupEnv("REDIS_ADDRESS")
	if !nonempty {
		log.Fatal("Env var REDIS_ADDRESS is not set.")
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Redis connection failed:", err)
	}

	redisClient.XGroupCreateMkStream(ctx, streamLatex, groupLatex, "0").Err()
	redisClient.XGroupCreateMkStream(ctx, streamTypst, groupTypst, "0").Err()

	os.MkdirAll("rendered", 0755)

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/render", renderHandler)
	http.HandleFunc("/status/", statusHandler)
	http.HandleFunc("/result/", resultHandler)
	http.Handle("/rendered/", http.StripPrefix("/rendered/", http.FileServer(http.Dir("./rendered"))))

	log.Println("API listening on http:/localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

