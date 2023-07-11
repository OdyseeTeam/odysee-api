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
	switch s3cfg.Flavor {
	case "aws", "":
		return newS3ClientV2AWS(s3cfg)
	case "ovh":
		return newS3ClientV2OVH(s3cfg)
	case "minio":
		return newS3ClientV2Minio(s3cfg)
	default:
		return nil, fmt.Errorf("invalid s3 flavor: %s", s3cfg.Flavor)
	}
}

func newS3ClientV2AWS(s3cfg S3Config) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(s3cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s3cfg.Key, s3cfg.Secret, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load aws sdk configuration: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	if s3cfg.VerifyBucket {
		input := &s3.HeadBucketInput{
			Bucket: aws.String(s3cfg.Bucket),
		}

		_, err = client.HeadBucket(context.TODO(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to verify bucket (%+v): %w", s3cfg, err)
		}
	}
	return client, nil
}

func newS3ClientV2Minio(s3cfg S3Config) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(s3cfg.Region),
		config.WithEndpointResolver(aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
			return aws.Endpoint{URL: s3cfg.Endpoint}, nil
		})),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s3cfg.Key, s3cfg.Secret, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load aws sdk configuration: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true })
	if s3cfg.VerifyBucket {
		input := &s3.HeadBucketInput{
			Bucket: aws.String(s3cfg.Bucket),
		}

		_, err = client.HeadBucket(context.TODO(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to verify bucket (%+v): %w", s3cfg, err)
		}
	}
	return client, nil
}

func newS3ClientV2OVH(s3cfg S3Config) (*s3.Client, error) {
	var endpointResolver = func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:           s3cfg.Endpoint,
			SigningRegion: s3cfg.Region,
		}, nil
	}

	// Create a custom resolver for OVH S3 credentials
	credentialsProvider := credentials.StaticCredentialsProvider{
		Value: aws.Credentials{
			AccessKeyID:     s3cfg.Key,           // Replace with your OVH S3 access key
			SecretAccessKey: s3cfg.Secret,        // Replace with your OVH S3 secret key
			Source:          "StaticCredentials", // OVH S3 requires this source value
		},
	}
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(endpointResolver)),
		config.WithCredentialsProvider(credentialsProvider),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load aws sdk configuration: %w", err)
	}

	client := s3.NewFromConfig(cfg)
	if s3cfg.VerifyBucket {
		input := &s3.HeadBucketInput{
			Bucket: aws.String(s3cfg.Bucket),
		}

		_, err = client.HeadBucket(context.TODO(), input)
		if err != nil {
			return nil, fmt.Errorf("failed to verify bucket (%+v): %w", s3cfg, err)
		}
	}
	return client, nil
}
