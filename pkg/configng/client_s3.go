package configng

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func NewS3Client(s3cfg S3Config) (*s3.S3, error) {
	cfg := aws.NewConfig().
		WithCredentials(credentials.NewStaticCredentials(s3cfg.Key, s3cfg.Secret, "")).
		WithRegion(s3cfg.Region)

	if s3cfg.Endpoint != "" {
		cfg = cfg.WithEndpoint(s3cfg.Endpoint)
	}

	if s3cfg.Flavor == "minio" || s3cfg.Flavor == "ovh" {
		cfg = cfg.WithS3ForcePathStyle(true)
	}

	sess, err := session.NewSession(cfg)
	if err != nil {
		panic(fmt.Errorf("unable to create AWS session: %w", err))
	}
	client := s3.New(sess)
	return client, nil
}
