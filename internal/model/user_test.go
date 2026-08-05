package model

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCheckPassword_CorrectPassword 验证正确密码能通过校验。
func TestCheckPassword_CorrectPassword(t *testing.T) {
	hashed, err := HashPassword("mySecret123")
	assert.NoError(t, err)
	assert.True(t, CheckPassword("mySecret123", hashed))
}

// TestCheckPassword_WrongPassword 验证错误密码被拒绝。
// 这是本次修复的核心断言：密码错误必须返回 false，而不是触发上层基础设施错误。
func TestCheckPassword_WrongPassword(t *testing.T) {
	hashed, err := HashPassword("correctPassword")
	assert.NoError(t, err)
	assert.False(t, CheckPassword("wrongPassword", hashed))
}

// TestCheckPassword_EmptyPassword 验证空密码行为。
func TestCheckPassword_EmptyPassword(t *testing.T) {
	hashed, err := HashPassword("nonEmpty")
	assert.NoError(t, err)
	assert.False(t, CheckPassword("", hashed))
}

// TestCheckPassword_EmptyHash 验证空哈希返回 false 而非 panic。
// 防止数据库字段为空时引发服务崩溃。
func TestCheckPassword_EmptyHash(t *testing.T) {
	assert.False(t, CheckPassword("anyPassword", ""))
}

// TestHashPassword_NonEmpty 验证哈希结果非空且与原文不同。
func TestHashPassword_NonEmpty(t *testing.T) {
	hashed, err := HashPassword("test123")
	assert.NoError(t, err)
	assert.NotEmpty(t, hashed)
	assert.NotEqual(t, "test123", hashed)
}

// TestVerifyPassword_Signature 验证 VerifyPassword 返回三个值（(*User, bool, error)）。
// 这是一个编译期契约测试：如果有人意外把签名改回两个返回值，此测试将编译失败。
// 重点：err 非 nil 时 ok 必须为 false；ok 为 false 时 err 可能为 nil（认证失败）也可能非 nil（基础设施错误）。
func TestVerifyPassword_Signature(t *testing.T) {
	// 真正的编译期断言：将方法值赋值给显式类型的变量。
	// 若 VerifyPassword 签名被改为两个返回值，此行编译失败。
	var _ func(*UserModel, context.Context, string, string) (*User, bool, error) = (*UserModel).VerifyPassword
}
