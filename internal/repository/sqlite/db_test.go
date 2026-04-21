package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/xjock/sentinel-crawler/internal/domain"
)

func setupTestDB(t *testing.T) *ProductRepository {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return NewProductRepository(db)
}

func TestProductRepository_SaveAndFind(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	product := &domain.Product{
		ID:          "test-001",
		Name:        "S2A_test",
		Platform:    "S2A",
		ProductType: "L1C",
		SensingDate: time.Now(),
		Size:        1024,
		DownloadURL: "https://example.com/test",
		Checksum:    "abc123",
		Source:      "copernicus",
		Metadata:    map[string]string{"key": "value"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := repo.Save(ctx, product); err != nil {
		t.Fatalf("save: %v", err)
	}

	found, err := repo.FindByID(ctx, "test-001")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found == nil {
		t.Fatal("expected product, got nil")
	}
	if found.Name != "S2A_test" {
		t.Errorf("expected name S2A_test, got %s", found.Name)
	}

	exists, err := repo.Exists(ctx, "test-001")
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if !exists {
		t.Error("expected product to exist")
	}
}

func TestProductRepository_SaveBatch(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	products := []*domain.Product{
		{ID: "batch-1", Name: "P1", Platform: "S1A", Source: "copernicus", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "batch-2", Name: "P2", Platform: "S2A", Source: "copernicus", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	if err := repo.SaveBatch(ctx, products); err != nil {
		t.Fatalf("save batch: %v", err)
	}

	count, err := repo.Count(ctx, domain.ProductQuery{})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestProductRepository_FindByQuery(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	products := []*domain.Product{
		{ID: "q-1", Name: "P1", Platform: "S1A", ProductType: "GRD", Source: "copernicus", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "q-2", Name: "P2", Platform: "S2A", ProductType: "L1C", Source: "copernicus", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	repo.SaveBatch(ctx, products)

	results, err := repo.FindByQuery(ctx, domain.ProductQuery{Platform: "S1A"})
	if err != nil {
		t.Fatalf("find by query: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}
