package logic

import "go.mongodb.org/mongo-driver/bson"

// sensitiveTopN 每类敏感命中条目的默认 top-N，Phase 3.5 暂用常量。
// 后续若需参数化可在 detail 接口加 query 参数（如 ?sensitiveLimit=20）。
const sensitiveTopN = 10

// sensitiveInfoKeywords 命中"敏感信息"类漏洞的关键字。
// 用于扫描 {wsId}_vul 中 vul_name/tags 字段（小写匹配），
// 与 Phase 1 迁移脚本 tools/migrate_asset_target_meta.go:330 的 auto:info-leak 判定保持一致。
var sensitiveInfoKeywords = []string{
	"敏感信息",
	"info leak",
	"info-leak",
	"infoleak",
	"sensitive",
	"leak",
	"disclosure",
	"暴露",
	"泄露",
}

// sensitiveDirKeywords 命中"敏感目录/敏感文件"类漏洞的关键字。
// 同样用于 {wsId}_vul 的 vul_name/tags 字段小写匹配。
var sensitiveDirKeywords = []string{
	"敏感目录",
	"敏感文件",
	"dir-listing",
	"directory listing",
	"dir listing",
	"_backup",
	"backup",
	".git",
	".svn",
	".env",
	"dump",
	"exposed",
}

// keywordOrClause 把关键字数组转为 MongoDB $or 子句，匹配 vul_name 或 tags（大小写不敏感）。
// 返回 []interface{} 以便直接放入 bson.M 的 "$or" 字段。
func keywordOrClause(keywords []string) []interface{} {
	clause := make([]interface{}, 0, len(keywords)*2)
	for _, kw := range keywords {
		clause = append(clause,
			bson.M{"vul_name": bson.M{"$regex": kw, "$options": "i"}},
			bson.M{"tags": kw},
		)
	}
	return clause
}

