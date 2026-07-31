package user

import (
	"net/http"

	"cscan/api/internal/logic"
	"cscan/api/internal/svc"
	"cscan/api/internal/types"

	"github.com/zeromicro/go-zero/rest/httpx"
)

// UserOnboardingCompleteHandler 标记当前用户首次引导已完成（T4.2）
func UserOnboardingCompleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewUserOnboardingLogic(r.Context(), svcCtx)
		resp, err := l.UserOnboardingComplete()
		if err != nil {
			httpx.OkJson(w, &types.BaseResp{Code: 500, Msg: err.Error()})
			return
		}
		httpx.OkJson(w, resp)
	}
}
