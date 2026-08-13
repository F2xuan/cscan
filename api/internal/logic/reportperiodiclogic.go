package logic

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"

	"github.com/xuri/excelize/v2"
	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/bson"
)

// ReportPeriodicGenerateLogic 周期报告生成（日报/周报/月报）T5.1
type ReportPeriodicGenerateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReportPeriodicGenerateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReportPeriodicGenerateLogic {
	return &ReportPeriodicGenerateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReportPeriodicGenerateLogic) PeriodicGenerate(req *types.ReportPeriodicGenerateReq) (*types.ReportPeriodicGenerateResp, error) {
	period := strings.ToLower(strings.TrimSpace(req.Period))
	if period == "" {
		period = "weekly"
	}
	start, end, prevStart, prevEnd := derivePeriodRange(period, req.End)

	cur, err := l.aggregate(start, end)
	if err != nil {
		return &types.ReportPeriodicGenerateResp{Code: 500, Msg: "聚合失败: " + err.Error()}, nil
	}
	prev, err := l.aggregate(prevStart, prevEnd)
	if err != nil {
		logx.Errorf("[ReportPeriodic] prev aggregate failed: %v", err)
		prev = &aggResult{}
	}

	data := &types.ReportPeriodicData{
		Period:    period,
		Start:     start.Format("2006-01-02"),
		End:       end.Format("2006-01-02"),
		PrevStart: prevStart.Format("2006-01-02"),
		PrevEnd:   prevEnd.Format("2006-01-02"),
		NewAssets: cur.newAssets,
		NewVulns:  cur.newVulns,
		NewVulnsBySeverity: types.ReportPeriodicSeverityStat{
			Critical: cur.sev["critical"],
			High:     cur.sev["high"],
			Medium:   cur.sev["medium"],
			Low:      cur.sev["low"],
			Info:     cur.sev["info"],
			Unknown:  cur.sev["unknown"],
		},
		Fixed:    cur.fixed,
		TopItems: cur.topItems,
		Trend: types.ReportPeriodicTrend{
			NewAssetsDelta: cur.newAssets - prev.newAssets,
			NewVulnsDelta:  cur.newVulns - prev.newVulns,
			FixedDelta:     cur.fixed - prev.fixed,
		},
	}
	return &types.ReportPeriodicGenerateResp{Code: 0, Msg: "success", Data: data}, nil
}

// ReportPeriodicExportLogic 周期报告导出
type ReportPeriodicExportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReportPeriodicExportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReportPeriodicExportLogic {
	return &ReportPeriodicExportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReportPeriodicExportLogic) PeriodicExport(req *types.ReportPeriodicExportReq) ([]byte, string, error) {
	gen := NewReportPeriodicGenerateLogic(l.ctx, l.svcCtx)
	resp, err := gen.PeriodicGenerate(&types.ReportPeriodicGenerateReq{Period: req.Period, End: req.End})
	if err != nil {
		return nil, "", err
	}
	if resp.Code != 0 {
		return nil, "", fmt.Errorf("%s", resp.Msg)
	}
	data, err := buildPeriodicXLSX(resp.Data)
	if err != nil {
		return nil, "", err
	}
	filename := fmt.Sprintf("report_periodic_%s_%s.xlsx", resp.Data.Period, time.Now().Format("20060102150405"))
	return data, filename, nil
}

// ============== 内部聚合 ==============

type aggResult struct {
	newAssets int64
	newVulns  int64
	sev       map[string]int64
	fixed     int64
	topItems  []types.ReportPeriodicItem
}

func (l *ReportPeriodicGenerateLogic) aggregate(start, end time.Time) (*aggResult, error) {
	res := &aggResult{sev: map[string]int64{}}

	diffModel := model.NewScanDiffModel(l.svcCtx.MongoDB)
	added, err := diffModel.FindByTimeRange(l.ctx, start, end, model.ScanDiffTypeAsset, model.ScanDiffChangeAdded)
	if err != nil {
		return nil, err
	}
	res.newAssets = int64(len(added))

	resolved, err := diffModel.FindByTimeRange(l.ctx, start, end, model.ScanDiffTypeVul, model.ScanDiffChangeResolved)
	if err != nil {
		return nil, err
	}
	res.fixed = int64(len(resolved))

	vulModel := model.NewVulModel(l.svcCtx.MongoDB)
	sev, err := vulModel.StatBySeverityInRange(l.ctx, start, end)
	if err != nil {
		return nil, err
	}
	res.sev = sev
	for _, c := range sev {
		res.newVulns += c
	}

	certModel := model.NewCertModel(l.svcCtx.MongoDB)
	certs, err := certModel.Find(l.ctx, bson.M{}, nil)
	if err != nil {
		return nil, err
	}
	now := time.Now()

	// Top N 最紧急事项：本周期 critical/high 漏洞 + 过期/即将过期证书
	top := []types.ReportPeriodicItem{}
	vulFilter := bson.M{
		"create_time": bson.M{"$gte": start, "$lt": end},
		"severity":    bson.M{"$in": []string{"critical", "high"}},
	}
	vuls, verr := vulModel.Find(l.ctx, vulFilter, 0, 0)
	if verr == nil {
		sort.Slice(vuls, func(i, j int) bool {
			return periodicSeverityRank[vuls[i].Severity] > periodicSeverityRank[vuls[j].Severity]
		})
		limit := 10
		if len(vuls) < limit {
			limit = len(vuls)
		}
		for _, v := range vuls[:limit] {
			top = append(top, types.ReportPeriodicItem{
				Key:        v.Authority,
				Summary:    v.VulName,
				Severity:   v.Severity,
				RefType:    "vul",
				CreateTime: v.CreateTime.Local().Format("2006-01-02 15:04:05"),
			})
		}
	}
	for _, c := range certs {
		daysLeft := int(c.NotAfter.Sub(now).Hours() / 24)
		if daysLeft <= 7 {
			summary := "证书即将过期"
			sev := "high"
			if daysLeft < 0 {
				summary = "证书已过期"
				sev = "critical"
			}
			top = append(top, types.ReportPeriodicItem{
				Key:        c.Authority,
				Summary:    summary,
				Severity:   sev,
				RefType:    "cert",
				CreateTime: c.UpdateTime.Local().Format("2006-01-02 15:04:05"),
			})
		}
		if len(top) >= 15 {
			break
		}
	}
	res.topItems = top
	return res, nil
}

var periodicSeverityRank = map[string]int{
	"critical": 5, "high": 4, "medium": 3, "low": 2, "info": 1, "unknown": 0,
}

// derivePeriodRange 根据周期类型与截止日期推导当前周期与上一周期区间
// 修复 M-17：使用上海时区本地午夜、半开区间 [start, end) 等长不重叠。
//   - weekly 当前区间为 7 天（原实现覆盖 8 天，上一周期覆盖 14 天）
//   - monthly 当前区间为 1 个月，上一周期为前 1 个月（原实现覆盖约两个月）
//   - 修复 time.Now().Truncate(24h) 在 UTC 上截断导致上海时区 08:00 才是"今天午夜"的问题
func derivePeriodRange(period, endStr string) (start, end, prevStart, prevEnd time.Time) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	dayEnd := parseEndDate(endStr).In(loc)
	// dayEnd 为 [当前周期] 的右端点（不含），即当天 00:00；
	// 所有区间均为半开区间 [start, end)，长度与 period 等长、互不重叠
	switch period {
	case "daily":
		// 当前：[昨日 00:00, 今日 00:00)，上一：[前日 00:00, 昨日 00:00)
		end = dayEnd
		start = end.AddDate(0, 0, -1)
		prevEnd = start
		prevStart = prevEnd.AddDate(0, 0, -1)
	case "monthly":
		// 当前：[上月同日 00:00, 今日 00:00)，上一：[上上月同日 00:00, 上月同日 00:00)
		end = dayEnd
		start = end.AddDate(0, -1, 0)
		prevEnd = start
		prevStart = prevEnd.AddDate(0, -1, 0)
	default: // weekly
		// 当前：[7 天前 00:00, 今日 00:00)，上一：[14 天前 00:00, 7 天前 00:00)
		end = dayEnd
		start = end.AddDate(0, 0, -7)
		prevEnd = start
		prevStart = prevEnd.AddDate(0, 0, -7)
	}
	return
}

func parseEndDate(endStr string) time.Time {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	if endStr != "" {
		if t, err := time.ParseInLocation("2006-01-02", endStr, loc); err == nil {
			return t
		}
	}
	// 修复 M-17：使用上海时区本地午夜，而非 UTC Truncate(24h) 造成的 08:00 偏移
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
}

// buildPeriodicXLSX 生成周期报告 Excel（含趋势对比），沿用现有导出通道（excelize）
func buildPeriodicXLSX(data *types.ReportPeriodicData) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	f.SetSheetName("Sheet1", "概览")
	rows := [][]interface{}{
		{"周期报告", strings.ToUpper(data.Period)},
		{"统计区间", fmt.Sprintf("%s ~ %s", data.Start, data.End)},
		{"对比区间", fmt.Sprintf("%s ~ %s", data.PrevStart, data.PrevEnd)},
		{"新增资产", data.NewAssets},
		{"新增漏洞", data.NewVulns},
		{"  严重(critical)", data.NewVulnsBySeverity.Critical},
		{"  高危(high)", data.NewVulnsBySeverity.High},
		{"  中危(medium)", data.NewVulnsBySeverity.Medium},
		{"  低危(low)", data.NewVulnsBySeverity.Low},
		{"  提示(info)", data.NewVulnsBySeverity.Info},
		{"  未知(unknown)", data.NewVulnsBySeverity.Unknown},
		{"已修复", data.Fixed},
		{"趋势-新增资产环比", data.Trend.NewAssetsDelta},
		{"趋势-新增漏洞环比", data.Trend.NewVulnsDelta},
		{"趋势-已修复环比", data.Trend.FixedDelta},
	}
	for i, r := range rows {
		for j, v := range r {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+1)
			f.SetCellValue("概览", cell, v)
		}
	}

	// 最紧急事项 Sheet
	f.NewSheet("最紧急事项")
	headers := []string{"类型", "目标", "摘要", "严重级别", "时间"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("最紧急事项", cell, h)
	}
	for i, it := range data.TopItems {
		row := i + 2
		vals := []interface{}{it.RefType, it.Key, it.Summary, it.Severity, it.CreateTime}
		for j, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(j+1, row)
			f.SetCellValue("最紧急事项", cell, v)
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
