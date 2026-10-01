package shared

import (
	"context"
	"log"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Environment struct {
	MINIO_INTERNAL_ENDPOINT string
	MINIO_EXTERNAL_ENDPOINT string
	MINIO_ROOT_USER        string
	MINIO_ROOT_PASSWORD        string
	MINIO_USE_SSL           string
}

var env Environment

func NewMinioClient(withPresigner bool) (*minio.Client, *minio.Client, error) {
	if err := envconfig.Process("", &env); err != nil {
		return nil, nil, err
	}

	client, err := minio.New(env.MINIO_INTERNAL_ENDPOINT, &minio.Options{
		Creds:  credentials.NewStaticV4(env.MINIO_ROOT_USER, env.MINIO_ROOT_PASSWORD, ""),
		Secure: false,
		// Secure: env.MINIO_USE_SSL == "true",
		Region: "us-east-1",
		// BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("Error for client: %w", err) 
	}

	// ping to check it works
	if _, err := client.ListBuckets(context.Background()); err != nil {
		return nil, nil, err
	}

	var presigner *minio.Client
	if withPresigner {
		// this client's only purpose is to generate presigned URLs (it doesnt need internet access; it cant reach localhost:9002)
		presigner, err = minio.New(env.MINIO_EXTERNAL_ENDPOINT, &minio.Options{
			Creds:  credentials.NewStaticV4(env.MINIO_ROOT_USER, env.MINIO_ROOT_PASSWORD, ""),
			Secure: env.MINIO_USE_SSL == "true",
			Region: "us-east-1",
			// BucketLookup: minio.BucketLookupPath,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("error for presigner: %w", err)
		}

	}

	log.Println("MinIO Client initialized successfully")

	return client, presigner, nil
}
