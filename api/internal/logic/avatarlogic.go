package logic

import (
	"context"
	"errors"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
)

// avatarUploadDir 头像本地存储目录（相对于工作目录）
const avatarUploadDir = "data/avatars"

// avatarURLPrefix 头像访问 URL 前缀
const avatarURLPrefix = "/static/avatars"

// avatarMaxSize 头像文件最大字节数（2MB）
const avatarMaxSize = 2 << 20

// avatarAllowedExts 允许的扩展名
var avatarAllowedExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}

// UserAvatarUploadLogic 头像上传逻辑
type UserAvatarUploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUserAvatarUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserAvatarUploadLogic {
	return &UserAvatarUploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserAvatarUploadLogic) Upload(header *multipart.FileHeader) (*types.AvatarUploadResp, error) {
	if header == nil {
		return nil, errors.New("头像文件为空")
	}
	if header.Size > avatarMaxSize {
		return nil, errors.New("头像文件不能超过2MB")
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !avatarAllowedExts[ext] {
		return nil, errors.New("仅支持 jpg/jpeg/png/gif/webp 格式")
	}

	src, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	// 解码以校验是否为真实图片（webp因标准库不支持，仅校验扩展名）
	if ext != ".webp" {
		var img image.Image
		switch ext {
		case ".jpg", ".jpeg":
			img, err = jpeg.Decode(src)
		case ".png":
			img, err = png.Decode(src)
		case ".gif":
			img, err = gif.Decode(src)
		}
		if err != nil || img == nil {
			return nil, errors.New("文件不是有效的图片")
		}
		// 重新打开用于后续读取
		_, _ = src.Seek(0, 0)
	}

	if err := os.MkdirAll(avatarUploadDir, 0o755); err != nil {
		return nil, err
	}

	filename := uuid.New().String() + ext
	destPath := filepath.Join(avatarUploadDir, filename)

	dst, err := os.Create(destPath)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	// 复制文件内容
	buf := make([]byte, 32*1024)
	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return nil, werr
			}
		}
		if rerr != nil {
			break
		}
	}

	avatarURL := avatarURLPrefix + "/" + filename
	return &types.AvatarUploadResp{
		Code:   0,
		Msg:    "上传成功",
		Avatar: avatarURL,
	}, nil
}

// UserUpdateAvatarLogic 更新当前登录用户头像逻辑
type UserUpdateAvatarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUserUpdateAvatarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UserUpdateAvatarLogic {
	return &UserUpdateAvatarLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UserUpdateAvatarLogic) Update(userId string, req *types.UserUpdateAvatarReq) (*types.BaseResp, error) {
	if userId == "" {
		return &types.BaseResp{Code: 401, Msg: "未认证"}, nil
	}
	if req.Avatar == "" {
		return &types.BaseResp{Code: 400, Msg: "头像地址不能为空"}, nil
	}

	// 仅允许 /static/avatars/ 前缀的相对路径
	if !strings.HasPrefix(req.Avatar, avatarURLPrefix) {
		return &types.BaseResp{Code: 400, Msg: "非法的头像地址"}, nil
	}

	// 写入数据库（仅保存文件名部分，避免暴露完整路径）
	filename := filepath.Base(req.Avatar)
	if filename == "" || strings.Contains(filename, "..") {
		return &types.BaseResp{Code: 400, Msg: "非法的头像地址"}, nil
	}

	avatarField := avatarURLPrefix + "/" + filename
	if err := l.svcCtx.UserModel.UpdateAvatar(l.ctx, userId, avatarField); err != nil {
		logx.Errorf("[UserUpdateAvatar] update failed: %v", err)
		return &types.BaseResp{Code: 500, Msg: "更新头像失败"}, nil
	}

	return &types.BaseResp{Code: 0, Msg: "更新成功"}, nil
}
