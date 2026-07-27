package user

import (
	"net/http"
	"os"
	"path/filepath"

	"cscan/api/internal/svc"

	"github.com/zeromicro/go-zero/rest/pathvar"
)

const avatarUploadDir = "data/avatars"

// AvatarStaticHandler 提供 /static/avatars/:filename 下的静态文件访问
// 直接走文件系统，禁止目录列表，防止路径穿越。
func AvatarStaticHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 仅允许 HEAD/GET
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// 从 pathvar 获取 :filename
		filename := ""
		if vars := pathvar.Vars(r); vars != nil {
			filename = vars["filename"]
		}
		// 回退：从 URL 截取
		if filename == "" {
			filename = filepath.Base(r.URL.Path)
		}
		if filename == "" || filename == "." || filename == ".." {
			http.NotFound(w, r)
			return
		}

		// 仅保留文件名，拼接为绝对路径
		filename = filepath.Base(filename)
		fullPath := filepath.Join(avatarUploadDir, filename)

		if _, err := os.Stat(fullPath); err != nil {
			http.NotFound(w, r)
			return
		}

		http.ServeFile(w, r, fullPath)
	}
}
