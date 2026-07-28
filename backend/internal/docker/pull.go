package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/image"
)

func (c *RealClient) Pull(ctx context.Context, ref string) error {
	ref = normalizeImageRef(ref)
	if ref == "" {
		return fmt.Errorf("image is empty")
	}
	c.logger.InfoContext(ctx, "docker pull", "image", ref)

	reader, err := c.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("image pull %s: %w", ref, err)
	}
	defer reader.Close()

	dec := json.NewDecoder(reader)
	for {
		var msg map[string]any
		if err := dec.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("image pull stream: %w", err)
		}
		if errMsg, ok := msg["error"].(string); ok && errMsg != "" {
			return fmt.Errorf("image pull %s: %s", ref, errMsg)
		}
	}
	return nil
}
