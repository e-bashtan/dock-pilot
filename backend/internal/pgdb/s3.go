package pgdb

import (
	"context"
	"fmt"
	"io"

	"github.com/ebash/dock-pilot/backend/internal/s3util"
)

type s3Config = s3util.Config

type s3Object = s3util.Object

func uploadToS3(ctx context.Context, cfg s3Config, key string, body io.Reader) (int64, error) {
	n, err := s3util.Upload(ctx, cfg, key, body)
	if err != nil {
		return n, err
	}
	return n, nil
}

func downloadFromS3(ctx context.Context, cfg s3Config, key string) (io.ReadCloser, error) {
	return s3util.Download(ctx, cfg, key)
}

func deleteFromS3(ctx context.Context, cfg s3Config, key string) error {
	return s3util.Delete(ctx, cfg, key)
}

func listFromS3(ctx context.Context, cfg s3Config, prefix string, limit int) ([]s3Object, error) {
	objs, err := s3util.List(ctx, cfg, prefix, limit, ".sql", ".sql.gz", ".gz")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return objs, nil
}
