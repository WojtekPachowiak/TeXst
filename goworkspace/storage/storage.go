package storage

import (
	"context"
	"github.com/kelseyhightower/envconfig"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"log"
)

type Environment struct {
	MINIO_INTERNAL_ENDPOINT string
	MINIO_EXTERNAL_ENDPOINT string
	MINIO_ACCESS_KEY        string
	MINIO_SECRET_KEY        string
	MINIO_USE_SSL           string
}

var env Environment

func InitMinio() (*minio.Client, *minio.Client, error) {
	err := envconfig.Process("", &env)
	if err != nil {
		return nil, nil, err
	}

	client, err := minio.New(env.MINIO_INTERNAL_ENDPOINT, &minio.Options{
		Creds:  credentials.NewStaticV4(env.MINIO_ACCESS_KEY, env.MINIO_SECRET_KEY, ""),
		Secure: env.MINIO_USE_SSL == "true",
		Region: "us-east-1",
		// BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, nil, err
	}

	// ping to check it works
	_, err = client.ListBuckets(context.Background())
	if err != nil {
		return nil, nil, err
	}

	// this client's only purpose is to generate presigned URLs (it doesnt need internet access; it cant reach localhost:9000)
	fakeClientForPresigning, err := minio.New(env.MINIO_EXTERNAL_ENDPOINT, &minio.Options{
		Creds:  credentials.NewStaticV4(env.MINIO_ACCESS_KEY, env.MINIO_SECRET_KEY, ""),
		Secure: env.MINIO_USE_SSL == "true",
		Region: "us-east-1",
		// BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, nil, err
	}

	log.Println("MinIO Clients initialized successfully")

	return client, fakeClientForPresigning, nil
}
