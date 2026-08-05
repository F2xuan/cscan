package logic

import (
	"context"
	"errors"
	"strings"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/internal/model"

	"gopkg.in/yaml.v3"
)

type ActiveFingerprintImportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewActiveFingerprintImportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ActiveFingerprintImportLogic {
	return &ActiveFingerprintImportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ActiveFingerprintImportLogic) ActiveFingerprintImport(req *types.ActiveFingerprintImportReq) (*types.ActiveFingerprintImportResp, error) {
	if req.Content == "" {
		return nil, errors.New("内容不能为空")
	}

	// 解析YAML（dir.yaml格式：AppName: ["/path1", "/path2"]）
	var dirYaml map[string][]string
	err := yaml.Unmarshal([]byte(req.Content), &dirYaml)
	if err != nil {
		return nil, errors.New("YAML解析失败: " + err.Error())
	}

	if len(dirYaml) == 0 {
		return nil, errors.New("未找到有效的指纹规则")
	}

	// 转换为ActiveFingerprint
	docs := make([]*model.ActiveFingerprint, 0, len(dirYaml))
	for name, paths := range dirYaml {
		// 过滤空路径
		validPaths := make([]string, 0, len(paths))
		for _, p := range paths {
			p = strings.TrimSpace(p)
			if p != "" {
				validPaths = append(validPaths, p)
			}
		}
		if len(validPaths) == 0 {
			continue
		}

		docs = append(docs, &model.ActiveFingerprint{
			Name:    name,
			Paths:   validPaths,
			Enabled: true,
		})
	}

	if len(docs) == 0 {
		return nil, errors.New("未找到有效的指纹规则")
	}

	// 批量导入
	inserted, updated, err := l.svcCtx.ActiveFingerprintModel.BulkUpsert(l.ctx, docs)
	if err != nil {
		return nil, err
	}

	return &types.ActiveFingerprintImportResp{
		Code:     0,
		Msg:      "导入成功",
		Imported: inserted,
		Updated:  updated,
	}, nil
}
