package storage

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"log"
)

type Environment struct {
	MINIO_ENDPOINT   string
	MINIO_ACCESS_KEY string
	MINIO_SECRET_KEY string
	MINIO_USE_SSL    string
}

var env Environment

func InitMinio() (*minio.Client, error) {
	err := envconfig.Process("", &env)
	if err != nil {
		return nil, err
	}

	client, err := minio.New(env.MINIO_ENDPOINT, &minio.Options{
		Creds:  credentials.NewStaticV4(env.MINIO_ACCESS_KEY, env.MINIO_SECRET_KEY, ""),
		Secure: env.MINIO_USE_SSL == "true",
	})
	if err != nil {
		return nil, err
	}

	log.Println("MinIO CLient initialized successfully")

	return client, nil
}
