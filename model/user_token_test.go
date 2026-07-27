package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGeneratePAT_PrefixAndUniqueness 验证：
//  1. token 总是以 `cscan_pat_` 前缀开头
//  2. 多次生成的 token 彼此不同（随机性）
func TestGeneratePAT_PrefixAndUniqueness(t *testing.T) {
	const n = 64
	set := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		tok, err := GeneratePAT()
		assert.NoError(t, err)
		assert.True(t, strings.HasPrefix(tok, PATPrefix), "token must start with %q, got %q", PATPrefix, tok)
		// 去掉前缀后应有足够长度（32 字节 base64url ≈ 43 字符）
		assert.Greater(t, len(tok), len(PATPrefix)+32)
		set[tok] = struct{}{}
	}
	assert.Equal(t, n, len(set), "all generated tokens must be unique")
}

// TestHashPAT_StableAndDistinct 验证同一明文 hash 稳定、不同明文 hash 不同
func TestHashPAT_StableAndDistinct(t *testing.T) {
	tok := "cscan_pat_abcdef1234567890"
	h1 := HashPAT(tok)
	h2 := HashPAT(tok)
	assert.Equal(t, h1, h2)

	// 与裸 sha256 一致
	sum := sha256.Sum256([]byte(tok))
	assert.Equal(t, hex.EncodeToString(sum[:]), h1)

	// 不同明文不同 hash
	h3 := HashPAT("cscan_pat_ZZZZZZZZZZZZZ")
	assert.NotEqual(t, h1, h3)
}

// TestPATPrefixOf 验证截取前 12 字符作为展示前缀
func TestPATPrefixOf(t *testing.T) {
	tok := "cscan_pat_abcdef"
	assert.Equal(t, "cscan_pat_ab", PATPrefixOf(tok))
	assert.Equal(t, "short", PATPrefixOf("short"), "短字符串原样返回")
}

// TestPATPrefixOf_HashNotReversibleFromPrefix 验证从前缀无法还原 hash
func TestPATPrefixOf_HashNotReversibleFromPrefix(t *testing.T) {
	tok := "cscan_pat_abcdefghijklmnop"
	prefix := PATPrefixOf(tok)
	hash := HashPAT(tok)
	assert.NotEqual(t, hash, HashPAT(prefix), "prefix 不应是完整 token 的 hash")
}

// 用 _ 表示忽略的内部变量
var _ = sha256.Size
