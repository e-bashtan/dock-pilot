package s3util

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Config struct {
	Endpoint       string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	ForcePathStyle bool
}

type Object struct {
	Key          string
	Size         int64
	LastModified time.Time
}

func NewClient(cfg Config) (*s3.Client, error) {
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	access := strings.TrimSpace(cfg.AccessKey)
	secret := strings.TrimSpace(cfg.SecretKey)
	if access == "" || secret == "" {
		return nil, fmt.Errorf("s3 access key and secret key are required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("s3 bucket is required")
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

func Upload(ctx context.Context, cfg Config, key string, body io.Reader) (int64, error) {
	client, err := NewClient(cfg)
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

func Download(ctx context.Context, cfg Config, key string) (io.ReadCloser, error) {
	client, err := NewClient(cfg)
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

func Delete(ctx context.Context, cfg Config, key string) error {
	client, err := NewClient(cfg)
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

// List returns objects under prefix matching any of the suffixes (case-insensitive).
// Empty suffixes means all non-directory keys.
func List(ctx context.Context, cfg Config, prefix string, limit int, suffixes ...string) ([]Object, error) {
	client, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	prefix = strings.Trim(prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	if limit <= 0 {
		limit = 100
	}

	var out []Object
	var token *string
	for {
		resp, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(cfg.Bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
			MaxKeys:           aws.Int32(1000),
		})
		if err != nil {
			return nil, fmt.Errorf("s3 list: %w", err)
		}
		for _, obj := range resp.Contents {
			key := aws.ToString(obj.Key)
			if key == "" || strings.HasSuffix(key, "/") {
				continue
			}
			if len(suffixes) > 0 && !matchSuffix(key, suffixes) {
				continue
			}
			item := Object{Key: key, Size: aws.ToInt64(obj.Size)}
			if obj.LastModified != nil {
				item.LastModified = obj.LastModified.UTC()
			}
			out = append(out, item)
		}
		if !aws.ToBool(resp.IsTruncated) || resp.NextContinuationToken == nil {
			break
		}
		token = resp.NextContinuationToken
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].LastModified.Equal(out[j].LastModified) {
			return out[i].Key > out[j].Key
		}
		return out[i].LastModified.After(out[j].LastModified)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Ping verifies credentials can access the bucket (ListObjects with MaxKeys=1).
func Ping(ctx context.Context, cfg Config) error {
	client, err := NewClient(cfg)
	if err != nil {
		return err
	}
	_, err = client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(cfg.Bucket),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return fmt.Errorf("s3 connection failed: %w", err)
	}
	return nil
}

func matchSuffix(key string, suffixes []string) bool {
	lower := strings.ToLower(key)
	for _, s := range suffixes {
		if strings.HasSuffix(lower, strings.ToLower(s)) {
			return true
		}
	}
	return false
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
