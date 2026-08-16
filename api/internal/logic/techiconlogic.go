package logic

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"cscan/api/internal/svc"
	"cscan/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	// techIconSourceBase 指纹库图标上游（Wappalyzer 社区延续版 webappanalyzer 的 jsdelivr CDN 镜像，
	// 与 wappalyzergo 内嵌指纹数据同源，图标文件名一一对应；CDN 在国内网络可达性远好于 raw.githubusercontent.com）。
	// 可通过环境变量 CSCAN_TECH_ICON_SRC 覆盖（如指向自建镜像或 raw.githubusercontent.com 原始地址）。
	techIconSourceBase = "https://cdn.jsdelivr.net/gh/enthec/webappanalyzer@main/src/images/icons"
	// techIconMaxBytes 单图标字节上限
	techIconMaxBytes = 512 << 10
	// techIconCacheTTL 解析结果本地缓存（含未命中负缓存，防止离线时反复请求上游）
	techIconCacheTTL = 30 * time.Minute
	// techIconMaxNameLen 技术名长度上限（防御异常入参）
	techIconMaxNameLen = 200
)

// techIconExts 图标扩展名白名单（上游 images/icons 目录仅含这些格式）
var techIconExts = map[string]bool{
	".svg": true, ".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".webp": true, ".ico": true,
}

type TechIconLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTechIconLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TechIconLogic {
	return &TechIconLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// GetIcon 返回技术名对应的图标（MongoDB 持久缓存，未命中时按需从上游拉取落库）。
// ok=false 表示无图标，调用方应回退为无图标的纯文本展示。
func (l *TechIconLogic) GetIcon(rawName string) (*model.TechIcon, bool) {
	rawName = strings.TrimSpace(rawName)
	normalized := svc.NormalizeTechName(rawName)
	if normalized == "" || len(normalized) > techIconMaxNameLen {
		return nil, false
	}

	// 1. MongoDB 持久缓存（离线部署后完全本地化）
	if cached, err := l.svcCtx.TechIconModel.FindByName(l.ctx, normalized); err != nil {
		l.Errorf("[TechIcon] cache lookup %q failed: %v", normalized, err)
	} else if cached != nil && len(cached.Data) > 0 {
		return cached, true
	}

	// 2. 本地内存缓存（singleflight 合并并发同名请求；未命中也缓存，避免离线时逐次回源）
	v, err := l.svcCtx.QueryCache.GetOrSetWithTTL("tech_icon:"+normalized, techIconCacheTTL, func() (interface{}, error) {
		return l.fetchAndStore(rawName, normalized), nil
	})
	if err != nil {
		return nil, false
	}
	if icon, ok := v.(*model.TechIcon); ok && icon != nil {
		return icon, true
	}
	return nil, false
}

// fetchAndStore 解析图标文件名并从上游下载，成功后写入 MongoDB 持久缓存。
// 返回 nil 表示该技术无可用图标（会进入本地负缓存）。
func (l *TechIconLogic) fetchAndStore(rawName, normalized string) *model.TechIcon {
	iconFile := l.resolveIconFile(rawName, normalized)
	if iconFile == "" {
		return nil
	}

	icon, err := l.downloadIcon(iconFile)
	if err != nil {
		l.Errorf("[TechIcon] download %q for %q: %v", iconFile, rawName, err)
		return nil
	}
	icon.Name = normalized
	icon.DisplayName = rawName

	// 落库失败不影响本次返回，下次请求会重试落库
	if err := l.svcCtx.TechIconModel.Upsert(l.ctx, icon); err != nil {
		l.Errorf("[TechIcon] persist %q failed: %v", normalized, err)
	}
	return icon
}

// resolveIconFile 优先取内嵌指纹映射；未命中时查自定义指纹库的 icon 字段；
// 仍未命中时对自定义指纹名做分词模糊兜底（如 "Tengine httpd" → 内置 Tengine 图标）。
// 自定义指纹的 icon 仅接受文件名（拼接到固定上游），不接受完整 URL，避免任意地址回源（SSRF）。
func (l *TechIconLogic) resolveIconFile(rawName, normalized string) string {
	if iconFile := l.svcCtx.TechIconMeta.ResolveIconFile(normalized); iconFile != "" {
		return iconFile
	}
	if fp, err := l.svcCtx.FingerprintModel.FindByName(l.ctx, rawName); err == nil && fp != nil {
		if iconFile := sanitizeIconFile(fp.Icon); iconFile != "" {
			return iconFile
		}
	}
	return l.svcCtx.TechIconMeta.ResolveIconFileFuzzy(rawName)
}

// sanitizeIconFile 校验图标文件名：非空、无路径分隔、无 ..、扩展名在白名单内
func sanitizeIconFile(iconFile string) string {
	iconFile = strings.TrimSpace(iconFile)
	if iconFile == "" || strings.ContainsAny(iconFile, "/\\") || strings.Contains(iconFile, "..") ||
		strings.Contains(iconFile, "://") {
		return ""
	}
	if !techIconExts[strings.ToLower(path.Ext(iconFile))] {
		return ""
	}
	return iconFile
}

// downloadIcon 从上游下载图标文件（CSCAN_TECH_ICON_SRC 可覆盖上游，用于内网镜像）
func (l *TechIconLogic) downloadIcon(iconFile string) (*model.TechIcon, error) {
	base := strings.TrimRight(os.Getenv("CSCAN_TECH_ICON_SRC"), "/")
	if base == "" {
		base = techIconSourceBase
	}
	reqURL := base + "/" + url.PathEscape(iconFile)

	req, err := http.NewRequestWithContext(l.ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	contentType := strings.TrimSpace(strings.SplitN(resp.Header.Get("Content-Type"), ";", 2)[0])
	if !strings.HasPrefix(contentType, "image/") {
		return nil, fmt.Errorf("unexpected content-type %q", contentType)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, techIconMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > techIconMaxBytes {
		return nil, fmt.Errorf("invalid icon size %d", len(data))
	}

	return &model.TechIcon{
		ContentType: contentType,
		Data:        data,
		Source:      reqURL,
	}, nil
}
