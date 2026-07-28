package pgdb

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Config struct {
	Endpoint       string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	ForcePathStyle bool
}

func newS3Client(cfg s3Config) (*s3.Client, error) {
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	access := strings.TrimSpace(cfg.AccessKey)
	secret := strings.TrimSpace(cfg.SecretKey)
	if access == "" || secret == "" {
		return nil, fmt.Errorf("%w: s3 access key and secret key are required", ErrInvalidInput)
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("%w: s3 bucket is required", ErrInvalidInput)
	}

	awsCfg := aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(access, secret, ""),
	}
	opts := []func(*s3.Options){
		func(o *s3.Options) {
			o.UsePathStyle = cfg.ForcePathStyle
		},
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint != "" {
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}
	return s3.NewFromConfig(awsCfg, opts...), nil
}

func uploadToS3(ctx context.Context, cfg s3Config, key string, body io.Reader) (int64, error) {
	client, err := newS3Client(cfg)
	if err != nil {
		return 0, err
	}
	uploader := manager.NewUploader(client)
	counter := &countingReader{r: body}
	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
		Body:   counter,
	})
	if err != nil {
		return counter.n, fmt.Errorf("s3 upload: %w", err)
	}
	return counter.n, nil
}

func downloadFromS3(ctx context.Context, cfg s3Config, key string) (io.ReadCloser, error) {
	client, err := newS3Client(cfg)
	if err != nil {
		return nil, err
	}
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 download: %w", err)
	}
	return out.Body, nil
}

func deleteFromS3(ctx context.Context, cfg s3Config, key string) error {
	client, err := newS3Client(cfg)
	if err != nil {
		return err
	}
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}
