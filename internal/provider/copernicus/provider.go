package copernicus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/xjock/sentinel-crawler/internal/config"
	"github.com/xjock/sentinel-crawler/internal/domain"
)

// odataResponse OData 搜索响应
type odataResponse struct {
	OdataContext string         `json:"@odata.context"`
	OdataCount   int            `json:"@odata.count"`
	Value        []odataProduct `json:"value"`
}

// odataProduct OData 产品结构
type odataProduct struct {
	ID               string `json:"Id"`
	Name             string `json:"Name"`
	ContentType      string `json:"ContentType"`
	ContentLength    int64  `json:"ContentLength"`
	OriginDate       string `json:"OriginDate"`
	PublicationDate  string `json:"PublicationDate"`
	ModificationDate string `json:"ModificationDate"`
	Online           bool   `json:"Online"`
	EvictionDate     string `json:"EvictionDate"`
	S3Path           string `json:"S3Path"`
	Checksum         []struct {
		Algorithm    string `json:"Algorithm"`
		Value        string `json:"Value"`
		ChecksumDate string `json:"ChecksumDate"`
	} `json:"Checksum"`
	ContentDate struct {
		Start string `json:"Start"`
		End   string `json:"End"`
	} `json:"ContentDate"`
	ProductionType           string          `json:"ProductionType"`
	ContentGeometry          json.RawMessage `json:"ContentGeometry"`
	ProductType              string          `json:"productType"`
	ProcessingLevel          string          `json:"processingLevel"`
	Platform                 string          `json:"platform"`
	PlatformSerialIdentifier string          `json:"platformSerialIdentifier"`
	InstrumentName           string          `json:"instrumentName"`
	InstrumentShortName      string          `json:"instrumentShortName"`
	OrbitDirection           string          `json:"orbitDirection"`
	PolarisationChannels     string          `json:"polarisationChannels"`
	SwathIdentifier          string          `json:"swathIdentifier"`
	CloudCover               float64         `json:"cloudCover"`
	SnowCover                float64         `json:"snowCover"`
}

// Provider Copernicus Data Space 实现
type Provider struct {
	cfg    config.CopernicusConfig
	auth   *authenticator
	client *http.Client
}

// NewProvider 创建 Copernicus Provider
func NewProvider(cfg config.CopernicusConfig) *Provider {
	client := &http.Client{Timeout: time.Duration(cfg.Timeout) * time.Second}
	return &Provider{
		cfg:    cfg,
		auth:   newAuthenticator(cfg, client),
		client: client,
	}
}

// Name 返回 Provider 名称
func (p *Provider) Name() string {
	return "copernicus"
}

// Search 搜索元数据
func (p *Provider) Search(ctx context.Context, query domain.SearchQuery) (*domain.SearchResult, error) {
	u, err := url.Parse(p.cfg.BaseURL + "/Products")
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	q := u.Query()
	q.Set("$count", "true")

	// OData $filter
	var filters []string
	if query.Platform != "" {
		filters = append(filters, fmt.Sprintf("contains(Name,'%s')", query.Platform))
	}
	if query.ProductType != "" {
		filters = append(filters, fmt.Sprintf("productType eq '%s'", query.ProductType))
	}
	if !query.SensingFrom.IsZero() {
		filters = append(filters, fmt.Sprintf("ContentDate/Start ge %s", query.SensingFrom.Format(time.RFC3339)))
	}
	if !query.SensingTo.IsZero() {
		filters = append(filters, fmt.Sprintf("ContentDate/Start le %s", query.SensingTo.Format(time.RFC3339)))
	}
	if query.Footprint.Type != "" && len(query.Footprint.Coordinates) > 0 {
		wkt := geometryToWKT(query.Footprint)
		if wkt != "" {
			filters = append(filters, fmt.Sprintf("OData.CSC.Intersects(area=geography'SRID=4326;%s')", wkt))
		}
	}

	if len(filters) > 0 {
		q.Set("$filter", strings.Join(filters, " and "))
	}

	// 分页
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > p.cfg.MaxResultsPerQuery && p.cfg.MaxResultsPerQuery > 0 {
		pageSize = p.cfg.MaxResultsPerQuery
	}
	q.Set("$top", strconv.Itoa(pageSize))

	page := query.Page
	if page < 1 {
		page = 1
	}
	q.Set("$skip", strconv.Itoa((page-1)*pageSize))

	q.Set("$orderby", "ContentDate/Start desc")
	u.RawQuery = q.Encode()

	req, err := p.newRequest(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("search returned %d: %s", resp.StatusCode, string(body))
	}

	var odata odataResponse
	if err := json.NewDecoder(resp.Body).Decode(&odata); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	products := make([]*domain.Product, 0, len(odata.Value))
	for _, op := range odata.Value {
		products = append(products, p.toDomainProduct(op))
	}

	hasMore := len(products) == pageSize && (page*pageSize) < odata.OdataCount

	return &domain.SearchResult{
		Products:   products,
		TotalCount: odata.OdataCount,
		Page:       page,
		PageSize:   pageSize,
		HasMore:    hasMore,
	}, nil
}

// GetProduct 获取单个产品
func (p *Provider) GetProduct(ctx context.Context, id string) (*domain.Product, error) {
	u := fmt.Sprintf("%s/Products('%s')", p.cfg.BaseURL, id)

	req, err := p.newRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get product request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get product returned %d", resp.StatusCode)
	}

	var op odataProduct
	if err := json.NewDecoder(resp.Body).Decode(&op); err != nil {
		return nil, fmt.Errorf("decode product: %w", err)
	}

	return p.toDomainProduct(op), nil
}

// Fetch 下载产品数据流
func (p *Provider) Fetch(ctx context.Context, product *domain.Product) (io.ReadCloser, error) {
	// 优先使用下载服务地址
	baseURL := p.cfg.DownloadBaseURL
	if baseURL == "" {
		baseURL = p.cfg.BaseURL
	}

	u := fmt.Sprintf("%s/odata/v1/Products('%s')/$value", baseURL, product.ID)

	req, err := p.newRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download returned %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// HealthCheck 检查数据源可用性
func (p *Provider) HealthCheck(ctx context.Context) error {
	u := p.cfg.BaseURL + "/Products?$top=1"
	req, err := p.newRequest(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned %d", resp.StatusCode)
	}
	return nil
}

// newRequest 创建带认证的 HTTP 请求
func (p *Provider) newRequest(ctx context.Context, method, urlStr string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, urlStr, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 认证（可选，搜索 API 公开访问）
	token, err := p.auth.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// toDomainProduct 转换为领域模型
func (p *Provider) toDomainProduct(op odataProduct) *domain.Product {
	product := &domain.Product{
		ID:          op.ID,
		Name:        op.Name,
		Platform:    op.Platform,
		ProductType: op.ProductType,
		Size:        op.ContentLength,
		Source:      "copernicus",
		Metadata:    make(map[string]string),
	}

	if op.ContentDate.Start != "" {
		if t, err := time.Parse(time.RFC3339, op.ContentDate.Start); err == nil {
			product.SensingDate = t
		}
	}
	if op.OriginDate != "" {
		if t, err := time.Parse(time.RFC3339, op.OriginDate); err == nil {
			product.IngestionDate = t
		}
	}

	// 几何
	if len(op.ContentGeometry) > 0 {
		product.Footprint.Type = "Polygon"
		// 简化处理：原样保存 GeoJSON
		product.Metadata["footprint_geojson"] = string(op.ContentGeometry)
	}

	// Checksum
	for _, cs := range op.Checksum {
		product.Checksum = cs.Value
		product.ChecksumAlgo = cs.Algorithm
	}

	// 元数据
	if op.ProcessingLevel != "" {
		product.Metadata["processingLevel"] = op.ProcessingLevel
	}
	if op.OrbitDirection != "" {
		product.Metadata["orbitDirection"] = op.OrbitDirection
	}
	if op.InstrumentName != "" {
		product.Metadata["instrumentName"] = op.InstrumentName
	}
	if op.CloudCover > 0 {
		product.Metadata["cloudCover"] = fmt.Sprintf("%.2f", op.CloudCover)
	}

	return product
}

// geometryToWKT 将 GeoJSON Geometry 转为 WKT（仅支持 Polygon）
func geometryToWKT(g domain.Geometry) string {
	if g.Type != "Polygon" || len(g.Coordinates) == 0 {
		return ""
	}

	// coordinates: []any{ []any{ [lon, lat], [lon, lat], ... } }
	rings, ok := g.Coordinates[0].([]any)
	if !ok || len(rings) == 0 {
		return ""
	}

	var coords []string
	for _, pt := range rings {
		ptArr, ok := pt.([]any)
		if !ok || len(ptArr) < 2 {
			continue
		}
		lon, lok := ptArr[0].(float64)
		lat, lak := ptArr[1].(float64)
		if !lok || !lak {
			continue
		}
		coords = append(coords, fmt.Sprintf("%.6f %.6f", lon, lat))
	}

	if len(coords) == 0 {
		return ""
	}

	return "POLYGON((" + strings.Join(coords, ", ") + "))"
}
