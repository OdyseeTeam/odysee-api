package configng

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func NewS3ClientV2(s3cfg S3Config) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(s3cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s3cfg.Key, s3cfg.Secret, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load aws sdk configuration: %w", err)
	}

	if s3cfg.Endpoint != "" {
		cfg.EndpointResolver = aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
			return aws.Endpoint{URL: s3cfg.Endpoint}, nil
		})
	}

	clientOptions := []func(*s3.Options){}
	if s3cfg.Minio {
		clientOptions = append(clientOptions, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	return s3.NewFromConfig(cfg, clientOptions...), nil
}
