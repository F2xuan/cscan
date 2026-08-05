package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"cscan/pkg/xerr"
)

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]interface{}{"key": "value"}

	Success(w, data)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}

	if resp.Code != xerr.OK {
		t.Errorf("期望 code=0，实际 %d", resp.Code)
	}
	if resp.Msg != "success" {
		t.Errorf("期望 msg=success，实际 %s", resp.Msg)
	}
	if resp.Data == nil {
		t.Error("期望 data 不为空")
	}
}

func TestSuccessWithMsg(t *testing.T) {
	w := httptest.NewRecorder()
	customMsg := "操作成功"

	SuccessWithMsg(w, customMsg)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != xerr.OK {
		t.Errorf("期望 code=0，实际 %d", resp.Code)
	}
	if resp.Msg != customMsg {
		t.Errorf("期望 msg=%s，实际 %s", customMsg, resp.Msg)
	}
}

func TestError_WithCodeError(t *testing.T) {
	w := httptest.NewRecorder()
	err := xerr.NewCodeError(xerr.UserNotFound)

	Error(w, err)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != xerr.UserNotFound {
		t.Errorf("期望 code=%d，实际 %d", xerr.UserNotFound, resp.Code)
	}
	if resp.Msg != xerr.GetMsg(xerr.UserNotFound) {
		t.Errorf("期望错误消息匹配")
	}
}

func TestError_WithGenericError(t *testing.T) {
	w := httptest.NewRecorder()
	err := errors.New("generic error")

	Error(w, err)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != xerr.ServerError {
		t.Errorf("期望 code=%d，实际 %d", xerr.ServerError, resp.Code)
	}
	if resp.Msg != "系统内部错误，请稍后重试" {
		t.Errorf("期望默认错误消息")
	}
}

func TestErrorWithCode(t *testing.T) {
	w := httptest.NewRecorder()

	ErrorWithCode(w, xerr.ParamError, "自定义参数错误")

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != xerr.ParamError {
		t.Errorf("期望 code=%d，实际 %d", xerr.ParamError, resp.Code)
	}
	if resp.Msg != "自定义参数错误" {
		t.Errorf("期望自定义消息，实际 %s", resp.Msg)
	}
}

func TestErrorWithCode_EmptyMsg(t *testing.T) {
	w := httptest.NewRecorder()

	ErrorWithCode(w, xerr.Unauthorized, "")

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != xerr.Unauthorized {
		t.Errorf("期望 code=%d，实际 %d", xerr.Unauthorized, resp.Code)
	}
	if resp.Msg == "" {
		t.Error("期望使用默认消息")
	}
}

func TestParamError(t *testing.T) {
	w := httptest.NewRecorder()

	ParamError(w, "缺少必填字段")

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != xerr.ParamError {
		t.Errorf("期望 code=%d，实际 %d", xerr.ParamError, resp.Code)
	}
	if resp.Msg != "缺少必填字段" {
		t.Errorf("期望自定义消息，实际 %s", resp.Msg)
	}
}

func TestParamError_EmptyMsg(t *testing.T) {
	w := httptest.NewRecorder()

	ParamError(w, "")

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != xerr.ParamError {
		t.Errorf("期望 code=%d，实际 %d", xerr.ParamError, resp.Code)
	}
	if resp.Msg == "" {
		t.Error("期望使用默认参数错误消息")
	}
}

func TestResponse_JSONStructure(t *testing.T) {
	resp := Response{
		Code: 0,
		Msg:  "success",
		Data: map[string]string{"key": "value"},
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("JSON 序列化失败: %v", err)
	}

	var unmarshaled Response
	if err := json.Unmarshal(b, &unmarshaled); err != nil {
		t.Fatalf("JSON 反序列化失败: %v", err)
	}

	if unmarshaled.Code != resp.Code {
		t.Errorf("code 不匹配")
	}
	if unmarshaled.Msg != resp.Msg {
		t.Errorf("msg 不匹配")
	}
}

func TestResponse_OmitEmptyData(t *testing.T) {
	resp := Response{
		Code: 0,
		Msg:  "success",
		Data: nil,
	}

	b, _ := json.Marshal(resp)
	str := string(b)

	if contains(str, "data") {
		t.Error("期望 data 字段被 omitempty 省略")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
