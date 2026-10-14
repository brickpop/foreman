package direct

import (
	"context"

	"github.com/brickpop/foreman/internal/runner"
)

// Images is the Direct engine's image builder: a no-op, since direct
// processes run the host's own tools.
type Images struct{}

var _ runner.Images = Images{}

// Ensure returns no image: the Direct engine ignores Spec.Image.
func (Images) Ensure(context.Context, string) (string, error) { return "", nil }

// Refresh has nothing to rebuild.
func (Images) Refresh(context.Context) error { return nil }

// Status lists no images.
func (Images) Status(context.Context) []runner.ImageStatus { return nil }
