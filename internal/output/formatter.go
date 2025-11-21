package output

import (
	"context"

	"devlog/internal/storage"
)

type ResultFormatter interface {
	Format(ctx context.Context, results []*storage.SearchResult, query string) (string, error)
}
