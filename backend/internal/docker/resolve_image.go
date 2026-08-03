package docker

import (
	"context"
	"fmt"
	"strings"
)

func (c *RealClient) ResolveLocalImage(ctx context.Context, preferred string, candidates ...string) (string, error) {
	seen := map[string]struct{}{}
	var refs []string
	add := func(ref string) {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return
		}
		ref = normalizeImageRef(ref)
		if _, ok := seen[ref]; ok {
			return
		}
		seen[ref] = struct{}{}
		refs = append(refs, ref)
	}
	add(preferred)
	for _, ref := range candidates {
		add(ref)
	}
	if len(refs) == 0 {
		return "", fmt.Errorf("no image candidates")
	}

	preferredNorm := normalizeImageRef(preferred)
	for _, ref := range refs {
		if _, _, err := c.cli.ImageInspectWithRaw(ctx, ref); err != nil {
			continue
		}
		if preferredNorm != "" && ref != preferredNorm {
			if err := c.cli.ImageTag(ctx, ref, preferredNorm); err != nil {
				c.logger.WarnContext(ctx, "retag legacy site image failed",
					"from", ref, "to", preferredNorm, "error", err)
				return ref, nil
			}
			c.logger.InfoContext(ctx, "retagged legacy site image", "from", ref, "to", preferredNorm)
			return preferredNorm, nil
		}
		return ref, nil
	}

	return "", fmt.Errorf("no local image for %s (tried %s) — redeploy the site", refs[0], strings.Join(refs, ", "))
}
