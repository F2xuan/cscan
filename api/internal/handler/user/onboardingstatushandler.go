package user

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// UserOnboardingStatusHandler 查询当前用户首次引导是否已完成（T4.2）
func UserOnboardingStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewUserOnboardingLogic(r.Context(), svcCtx)
		resp, err := l.UserOnboardingStatus()
		if err != nil {
			httpx.Error(w, err)
			return
		}
		httpx.OkJson(w, resp)
	}
}
