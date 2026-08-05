package xerr

import (
	"errors"
	"testing"
)

func TestCodeError_Error(t *testing.T) {
	err := &CodeError{
		Code: 10001,
		Msg:  "用户不存在",
	}

	expected := "code: 10001, msg: 用户不存在"
	if err.Error() != expected {
		t.Errorf("期望 %s，实际 %s", expected, err.Error())
	}
}

func TestNewCodeError(t *testing.T) {
	err := NewCodeError(UserNotFound)

	if err.Code != UserNotFound {
		t.Errorf("期望 code=%d，实际 %d", UserNotFound, err.Code)
	}
	if err.Msg == "" {
		t.Error("期望消息不为空")
	}
	if err.Msg != GetMsg(UserNotFound) {
		t.Errorf("期望消息为 %s，实际 %s", GetMsg(UserNotFound), err.Msg)
	}
}

func TestNewCodeErrorMsg(t *testing.T) {
	customMsg := "自定义错误消息"
	err := NewCodeErrorMsg(ParamError, customMsg)

	if err.Code != ParamError {
		t.Errorf("期望 code=%d，实际 %d", ParamError, err.Code)
	}
	if err.Msg != customMsg {
		t.Errorf("期望消息为 %s，实际 %s", customMsg, err.Msg)
	}
}

func TestNewParamError(t *testing.T) {
	err := NewParamError("缺少必填字段")

	if err.Code != ParamError {
		t.Errorf("期望 code=%d，实际 %d", ParamError, err.Code)
	}
	if err.Msg != "缺少必填字段" {
		t.Errorf("期望自定义消息，实际 %s", err.Msg)
	}
}

func TestNewParamError_EmptyMsg(t *testing.T) {
	err := NewParamError("")

	if err.Code != ParamError {
		t.Errorf("期望 code=%d，实际 %d", ParamError, err.Code)
	}
	if err.Msg == "" {
		t.Error("期望使用默认消息")
	}
	if err.Msg != GetMsg(ParamError) {
		t.Errorf("期望默认消息 %s，实际 %s", GetMsg(ParamError), err.Msg)
	}
}

func TestNewServerError(t *testing.T) {
	err := NewServerError("数据库连接失败")

	if err.Code != ServerError {
		t.Errorf("期望 code=%d，实际 %d", ServerError, err.Code)
	}
	if err.Msg != "数据库连接失败" {
		t.Errorf("期望自定义消息，实际 %s", err.Msg)
	}
}

func TestNewServerError_EmptyMsg(t *testing.T) {
	err := NewServerError("")

	if err.Code != ServerError {
		t.Errorf("期望 code=%d，实际 %d", ServerError, err.Code)
	}
	if err.Msg == "" {
		t.Error("期望使用默认消息")
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("资源不存在")

	if err.Code != NotFound {
		t.Errorf("期望 code=%d，实际 %d", NotFound, err.Code)
	}
	if err.Msg != "资源不存在" {
		t.Errorf("期望自定义消息，实际 %s", err.Msg)
	}
}

func TestNewNotFoundError_EmptyMsg(t *testing.T) {
	err := NewNotFoundError("")

	if err.Code != NotFound {
		t.Errorf("期望 code=%d，实际 %d", NotFound, err.Code)
	}
	if err.Msg == "" {
		t.Error("期望使用默认消息")
	}
}

func TestCodeError_AsError(t *testing.T) {
	var err error = NewCodeError(UserNotFound)

	var codeErr *CodeError
	if !errors.As(err, &codeErr) {
		t.Error("期望能够使用 errors.As 转换")
	}

	if codeErr.Code != UserNotFound {
		t.Errorf("转换后 code 不匹配")
	}
}

func TestCodeError_IsError(t *testing.T) {
	err := NewCodeError(Unauthorized)

	if err == nil {
		t.Error("CodeError 应实现 error 接口")
	}

	errorStr := err.Error()
	if errorStr == "" {
		t.Error("Error() 方法应返回非空字符串")
	}
}

func TestAllPredefinedErrors(t *testing.T) {
	testCases := []struct {
		name string
		code int
	}{
		{"OK", OK},
		{"ParamError", ParamError},
		{"Unauthorized", Unauthorized},
		{"Forbidden", Forbidden},
		{"NotFound", NotFound},
		{"ServerError", ServerError},
		{"UserNotFound", UserNotFound},
		{"UserPasswordError", UserPasswordError},
		{"UserDisabled", UserDisabled},
		{"TaskNotFound", TaskNotFound},
		{"TaskStatusError", TaskStatusError},
		{"WorkspaceNotFound", WorkspaceNotFound},
		{"AssetNotFound", AssetNotFound},
		{"VulNotFound", VulNotFound},
		{"FingerprintNotFound", FingerprintNotFound},
		{"PocNotFound", PocNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewCodeError(tc.code)
			if err.Code != tc.code {
				t.Errorf("期望 code=%d，实际 %d", tc.code, err.Code)
			}
			if err.Msg == "" {
				t.Errorf("code=%d 的默认消息不应为空", tc.code)
			}
		})
	}
}

func TestGetMsg_UnknownCode(t *testing.T) {
	unknownCode := 99999
	msg := GetMsg(unknownCode)

	if msg == "" {
		t.Error("未知错误码应返回默认消息")
	}
}
