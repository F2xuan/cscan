package techicon

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/pkg/response"
)

// TechIconHandler GET /api/v1/tech/icon?name=Nginx
// 公开只读端点：技术图标为通用公开 Logo 数据，且 <img> 标签无法携带 Authorization 头，
// 因此与 /static/avatars 一样放在免认证路由组。
func TechIconHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			response.ParamError(w, "name is required")
			return
		}

		l := logic.NewTechIconLogic(r.Context(), svcCtx)
		icon, ok := l.GetIcon(name)
		if !ok {
			// 短时缓存 404，避免前端对无图标技术反复触发回源
			w.Header().Set("Cache-Control", "public, max-age=3600")
			http.NotFound(w, r)
			return
		}

		// 图标内容不可变，允许浏览器/代理长缓存
		w.Header().Set("Content-Type", icon.ContentType)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		w.Write(icon.Data)
	}
}
