package user

import (
	"errors"
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/middleware"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"
	"cscan/pkg/response"
	"cscan/pkg/xerr"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// UserAvatarUploadHandler 用户头像上传
// 管理员在用户管理中为任意用户上传头像；普通用户也可上传自己的头像。
func UserAvatarUploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 限制单次上传体积（5MB）
		if err := r.ParseMultipartForm(5 << 20); err != nil {
			response.ParamError(w, "头像文件过大或格式错误")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			response.ParamError(w, "请上传头像文件")
			return
		}
		defer file.Close()

		l := logic.NewUserAvatarUploadLogic(r.Context(), svcCtx)
		resp, err := l.Upload(header)
		if err != nil {
			logx.Errorf("[UserAvatarUpload] upload failed: %v", err)
			response.ErrorWithCode(w, xerr.ServerError, err.Error())
			return
		}
		httpx.OkJson(w, resp)
	}
}

// UserUpdateAvatarHandler 更新当前登录用户的头像（自助）
func UserUpdateAvatarHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UserUpdateAvatarReq
		if err := httpx.Parse(r, &req); err != nil {
			response.ParamError(w, err.Error())
			return
		}

		userId := middleware.GetUserId(r.Context())
		if userId == "" {
			response.Error(w, errors.New("未认证"))
			return
		}

		l := logic.NewUserUpdateAvatarLogic(r.Context(), svcCtx)
		resp, err := l.Update(userId, &req)
		if err != nil {
			response.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}
