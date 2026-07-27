package logic

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"cscan/rpc/task/internal/svc"
	"cscan/rpc/task/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RequestResourceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRequestResourceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RequestResourceLogic {
	return &RequestResourceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 请求资源文件
// 安全说明:对 category 和 name 做严格校验,防止路径遍历攻击(如 ../../etc/passwd)
func (l *RequestResourceLogic) RequestResource(in *pb.RequestResourceReq) (*pb.RequestResourceResp, error) {
	category := in.Category
	name := in.Name

	if category == "" || name == "" {
		return &pb.RequestResourceResp{
			Path: "",
			Hash: "",
			Data: nil,
		}, nil
	}

	// 安全校验:拒绝包含路径分隔符或 ".." 的输入,防止路径遍历
	if strings.ContainsAny(category, `/\`) || strings.Contains(category, "..") ||
		strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		l.Logger.Errorf("RequestResource: path escape attempt: category=%q name=%q", category, name)
		return &pb.RequestResourceResp{
			Path: "",
			Hash: "",
			Data: nil,
		}, nil
	}

	// 构建资源文件路径,并取绝对路径进行二次校验
	// 资源文件存放在 resources/<category>/<name>
	resourceRoot, err := filepath.Abs("resources")
	if err != nil {
		l.Logger.Errorf("RequestResource: failed to resolve resource root: %v", err)
		return &pb.RequestResourceResp{
			Path: "",
			Hash: "",
			Data: nil,
		}, nil
	}

	resourcePath := filepath.Join(resourceRoot, category, name)
	absPath, err := filepath.Abs(resourcePath)
	if err != nil {
		l.Logger.Errorf("RequestResource: failed to resolve abs path: %v", err)
		return &pb.RequestResourceResp{
			Path: "",
			Hash: "",
			Data: nil,
		}, nil
	}

	// 二次校验:确保解析后的绝对路径仍在 resources 目录内
	// 使用 HasPrefix + 路径分隔符防止 "resources-evil" 这类前缀绕过
	expectedPrefix := resourceRoot + string(filepath.Separator)
	if absPath != resourceRoot && !strings.HasPrefix(absPath+string(filepath.Separator), expectedPrefix) {
		l.Logger.Errorf("RequestResource: resolved path escapes resource root: %s", absPath)
		return &pb.RequestResourceResp{
			Path: "",
			Hash: "",
			Data: nil,
		}, nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		l.Logger.Infof("RequestResource: resource not found: %s", absPath)
		return &pb.RequestResourceResp{
			Path: "",
			Hash: "",
			Data: nil,
		}, nil
	} else if err != nil {
		l.Logger.Errorf("RequestResource: stat failed: %v", err)
		return &pb.RequestResourceResp{
			Path: "",
			Hash: "",
			Data: nil,
		}, nil
	}

	// 读取文件内容
	data, err := os.ReadFile(absPath)
	if err != nil {
		l.Logger.Errorf("RequestResource: failed to read resource: %v", err)
		return &pb.RequestResourceResp{
			Path: "",
			Hash: "",
			Data: nil,
		}, nil
	}

	// 计算MD5哈希
	hash := md5.Sum(data)
	hashStr := hex.EncodeToString(hash[:])

	// 返回相对路径,避免泄漏服务器绝对路径
	relPath := filepath.ToSlash(filepath.Join("resources", category, name))
	return &pb.RequestResourceResp{
		Path: relPath,
		Hash: hashStr,
		Data: data,
	}, nil
}
