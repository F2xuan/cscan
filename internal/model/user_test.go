package model

import (
	"testing"
)

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"空字符串", "", true},
		{"少于8位", "Abc123", true},
		{"正好8位合法", "Abc12345", false},
		{"缺大写", "abc12345", true},
		{"缺小写", "ABC12345", true},
		{"缺数字", "Abcdefgh", true},
		{"全部满足", "Test1234", false},
		{"包含特殊字符", "Test@123", false},
		{"超长密码", "Test1234567890ABCDEF", false},
		{"Unicode字符", "Test123中文", false},
		{"Unicode大写数字", "Тест1234", true}, // 西里尔字母Т不匹配A-Z，验证前后端一致性
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePasswordStrength(%q) error=%v, wantErr=%v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestHashPassword(t *testing.T) {
	password := "Test1234"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if len(hash) == 0 {
		t.Error("Hash should not be empty")
	}
	if hash == password {
		t.Error("Hash should not equal plaintext")
	}
}

func TestCheckPassword(t *testing.T) {
	password := "Test1234"
	hash, _ := HashPassword(password)

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"正确密码", password, true},
		{"错误密码", "Wrong123", false},
		{"空字符串", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckPassword(tt.input, hash)
			if result != tt.expected {
				t.Errorf("CheckPassword(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
