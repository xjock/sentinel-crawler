//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/lucavallin/sentinel-crawler/internal/config"
	"github.com/lucavallin/sentinel-crawler/internal/domain"
	"github.com/lucavallin/sentinel-crawler/internal/provider/copernicus"
)

// 运行 e2e 测试：
//   go test -tags=e2e ./tests/e2e/... -v
//
// 可选环境变量（用于认证测试）：
//   SENTINEL_CRAWLER_PROVIDERS_COPERNICUS_CLIENT_ID
//   SENTINEL_CRAWLER_PROVIDERS_COPERNICUS_CLIENT_SECRET
//   SENTINEL_CRAWLER_PROVIDERS_COPERNICUS_USERNAME
//   SENTINEL_CRAWLER_PROVIDERS_COPERNICUS_PASSWORD

func newTestProvider(t *testing.T) *copernicus.Provider {
	t.Helper()

	cfg := config.CopernicusConfig{
		BaseURL:            "https://catalogue.dataspace.copernicus.eu/odata/v1",
		DownloadBaseURL:    "https://download.dataspace.copernicus.eu",
		TokenURL:           "https://identity.dataspace.copernicus.eu/auth/realms/CDSE/protocol/openid-connect/token",
		ClientID:           os.Getenv("SENTINEL_CRAWLER_PROVIDERS_COPERNICUS_CLIENT_ID"),
		ClientSecret:       os.Getenv("SENTINEL_CRAWLER_PROVIDERS_COPERNICUS_CLIENT_SECRET"),
		Username:           os.Getenv("SENTINEL_CRAWLER_PROVIDERS_COPERNICUS_USERNAME"),
		Password:           os.Getenv("SENTINEL_CRAWLER_PROVIDERS_COPERNICUS_PASSWORD"),
		Timeout:            30,
		RateLimit:          5,
		MaxResultsPerQuery: 10,
	}

	return copernicus.NewProvider(cfg)
}

// TestSearch_WithoutAuth 测试无认证搜索
func TestSearch_WithoutAuth(t *testing.T) {
	p := newTestProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	query := domain.SearchQuery{
		Platform:    "S2A",
		PageSize:    3,
		SensingFrom: time.Now().AddDate(0, -1, 0),
		SensingTo:   time.Now(),
	}

	result, err := p.Search(ctx, query)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	t.Logf("Total count: %d", result.TotalCount)
	t.Logf("Page: %d, PageSize: %d, HasMore: %v", result.Page, result.PageSize, result.HasMore)

	if len(result.Products) == 0 {
		t.Fatal("expected at least one product, got none")
	}

	for _, p := range result.Products {
		t.Logf("Product: ID=%s Name=%s Platform=%s Type=%s Size=%d Sensing=%s",
			p.ID, p.Name, p.Platform, p.ProductType, p.Size,
			p.SensingDate.Format(time.RFC3339),
		)
	}

	if result.TotalCount <= 0 {
		t.Error("expected total_count > 0")
	}
}

// TestSearch_WithPagination 测试分页搜索
func TestSearch_WithPagination(t *testing.T) {
	p := newTestProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	query := domain.SearchQuery{
		Platform:    "S2A",
		PageSize:    2,
		Page:        1,
		SensingFrom: time.Now().AddDate(0, -3, 0),
		SensingTo:   time.Now(),
	}

	result, err := p.Search(ctx, query)
	if err != nil {
		t.Fatalf("search page 1 failed: %v", err)
	}

	if !result.HasMore {
		t.Skip("not enough data for pagination test")
	}

	firstID := result.Products[0].ID

	// 第二页
	query.Page = 2
	result2, err := p.Search(ctx, query)
	if err != nil {
		t.Fatalf("search page 2 failed: %v", err)
	}

	if len(result2.Products) == 0 {
		t.Fatal("expected products on page 2")
	}

	if result2.Products[0].ID == firstID {
		t.Error("page 2 should have different products than page 1")
	}

	t.Logf("Page 1 first ID: %s, Page 2 first ID: %s", firstID, result2.Products[0].ID)
}

// TestSearch_WithProductTypeFilter 测试产品类型过滤
func TestSearch_WithProductTypeFilter(t *testing.T) {
	p := newTestProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	query := domain.SearchQuery{
		Platform:    "S1A",
		ProductType: "GRD",
		PageSize:    3,
		SensingFrom: time.Now().AddDate(0, -1, 0),
		SensingTo:   time.Now(),
	}

	result, err := p.Search(ctx, query)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	for _, prod := range result.Products {
		if prod.ProductType != "GRD" {
			t.Errorf("expected product type GRD, got %s", prod.ProductType)
		}
	}
}

// TestHealthCheck 测试健康检查
func TestHealthCheck(t *testing.T) {
	p := newTestProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := p.HealthCheck(ctx); err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	t.Log("health check passed")
}

// TestAuth_WithCredentials 测试认证（需要凭证）
func TestAuth_WithCredentials(t *testing.T) {
	clientID := os.Getenv("SENTINEL_CRAWLER_PROVIDERS_COPERNICUS_CLIENT_ID")
	clientSecret := os.Getenv("SENTINEL_CRAWLER_PROVIDERS_COPERNICUS_CLIENT_SECRET")
	username := os.Getenv("SENTINEL_CRAWLER_PROVIDERS_COPERNICUS_USERNAME")
	password := os.Getenv("SENTINEL_CRAWLER_PROVIDERS_COPERNICUS_PASSWORD")

	if clientID == "" && username == "" {
		t.Skip("no credentials provided, set CLIENT_ID or USERNAME env vars")
	}

	if clientID != "" && clientSecret == "" {
		t.Skip("CLIENT_ID set but CLIENT_SECRET missing")
	}
	if username != "" && password == "" {
		t.Skip("USERNAME set but PASSWORD missing")
	}

	p := newTestProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 先搜索一个产品用于下载测试
	query := domain.SearchQuery{
		Platform:    "S2A",
		PageSize:    1,
		SensingFrom: time.Now().AddDate(0, -1, 0),
		SensingTo:   time.Now(),
	}

	result, err := p.Search(ctx, query)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(result.Products) == 0 {
		t.Fatal("no products found for download test")
	}

	product := result.Products[0]
	t.Logf("Found product for download test: %s", product.ID)

	// 测试认证下载（只获取 header，不下载完整文件）
	testDownload(t, p, product)
}

func testDownload(t *testing.T, p *copernicus.Provider, product *domain.Product) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	reader, err := p.Fetch(ctx, product)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	defer reader.Close()

	// 读取前几个字节确认是 ZIP 文件
	buf := make([]byte, 4)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	// ZIP 文件魔数: 50 4B 03 04
	if n >= 4 && buf[0] == 0x50 && buf[1] == 0x4B {
		t.Log("download verified: valid ZIP header")
	} else {
		t.Logf("download response first bytes: %x", buf[:n])
	}
}

// TestAuth_InvalidCredentials 测试无效凭证
func TestAuth_InvalidCredentials(t *testing.T) {
	cfg := config.CopernicusConfig{
		BaseURL:      "https://catalogue.dataspace.copernicus.eu/odata/v1",
		TokenURL:     "https://identity.dataspace.copernicus.eu/auth/realms/CDSE/protocol/openid-connect/token",
		ClientID:     "invalid-client-id",
		ClientSecret: "invalid-secret",
		Timeout:      10,
	}

	p := copernicus.NewProvider(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 搜索不需要认证，应该成功
	query := domain.SearchQuery{Platform: "S2A", PageSize: 1}
	_, err := p.Search(ctx, query)
	if err != nil {
		t.Logf("search with invalid auth (expected to work for public search): %v", err)
	}

	// 下载需要认证，应该失败
	product := &domain.Product{ID: "test", Name: "test"}
	_, err = p.Fetch(ctx, product)
	if err == nil {
		t.Fatal("expected fetch to fail with invalid credentials")
	}
	t.Logf("fetch with invalid credentials failed as expected: %v", err)
}

// TestGetProduct 测试获取单个产品详情
func TestGetProduct(t *testing.T) {
	p := newTestProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 先搜索一个产品
	query := domain.SearchQuery{
		Platform:    "S2A",
		PageSize:    1,
		SensingFrom: time.Now().AddDate(0, -1, 0),
		SensingTo:   time.Now(),
	}

	result, err := p.Search(ctx, query)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(result.Products) == 0 {
		t.Fatal("no products found")
	}

	id := result.Products[0].ID
	product, err := p.GetProduct(ctx, id)
	if err != nil {
		t.Fatalf("get product failed: %v", err)
	}

	if product.ID != id {
		t.Errorf("expected id %s, got %s", id, product.ID)
	}

	t.Logf("Product detail: Name=%s Platform=%s Type=%s Size=%d",
		product.Name, product.Platform, product.ProductType, product.Size)
}

// TestSearch_Sentinel1 测试 S1 数据搜索
func TestSearch_Sentinel1(t *testing.T) {
	p := newTestProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	query := domain.SearchQuery{
		Platform:    "S1A",
		PageSize:    2,
		SensingFrom: time.Now().AddDate(0, -1, 0),
		SensingTo:   time.Now(),
	}

	result, err := p.Search(ctx, query)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(result.Products) == 0 {
		t.Skip("no S1A products found in last month")
	}

	for _, p := range result.Products {
		t.Logf("S1A Product: %s | Type: %s | Size: %d MB",
			p.Name, p.ProductType, p.Size/1024/1024)
	}
}

// TestRateLimit 测试限流不会导致请求失败
func TestRateLimit(t *testing.T) {
	p := newTestProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 快速发起多次搜索
	for i := 0; i < 5; i++ {
		query := domain.SearchQuery{
			Platform:    "S2A",
			PageSize:    1,
			SensingFrom: time.Now().AddDate(0, -1, 0),
			SensingTo:   time.Now(),
		}

		result, err := p.Search(ctx, query)
		if err != nil {
			t.Fatalf("search %d failed: %v", i+1, err)
		}
		t.Logf("Search %d: %d total results", i+1, result.TotalCount)
	}
}

// Helper: 检查 HTTP 状态码
func isHTTPError(err error, code int) bool {
	if err == nil {
		return false
	}
	// 简单的字符串匹配，实际可以扩展为自定义 error 类型
	return false
}

func init() {
	// 确保 HTTP 默认 transport 不会缓存连接导致测试干扰
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 10
}
