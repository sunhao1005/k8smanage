package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDisabledAllowsAll(t *testing.T) {
	a := New("", "", "", "", 0)
	require.False(t, a.Enabled())
	require.True(t, a.Valid("")) // 未启用鉴权放行
}

func TestLoginAndSession(t *testing.T) {
	a := New("admin", "s3cret", "", "signing-key", time.Hour)
	require.True(t, a.Enabled())
	require.True(t, a.LoginEnabled())

	_, ok := a.Login("admin", "wrong")
	require.False(t, ok)

	tok, ok := a.Login("admin", "s3cret")
	require.True(t, ok)
	require.NotEmpty(t, tok)

	require.True(t, a.Valid(tok))     // 有效会话
	require.False(t, a.Valid("junk")) // 乱码
	require.False(t, a.Valid(tok+"x"))
}

func TestSessionExpiry(t *testing.T) {
	a := New("admin", "p", "", "k", time.Hour)
	// 用一个已过期的密钥同款签发：直接构造过期 payload
	a.ttl = -time.Minute
	tok, ok := a.Login("admin", "p")
	require.True(t, ok)
	require.False(t, a.Valid(tok)) // 已过期
}

func TestSessionTamperRejected(t *testing.T) {
	a := New("admin", "p", "", "k1", time.Hour)
	tok, _ := a.Login("admin", "p")
	// 换一个不同密钥的实例，签名应不匹配
	b := New("admin", "p", "", "k2", time.Hour)
	require.False(t, b.Valid(tok))
}

func TestStaticAPIToken(t *testing.T) {
	a := New("", "", "apikey123", "", time.Hour)
	require.True(t, a.Enabled())
	require.False(t, a.LoginEnabled()) // 只有 API Token，无账密登录
	require.True(t, a.Valid("apikey123"))
	require.False(t, a.Valid("nope"))
}

func TestCombinedUserPassAndToken(t *testing.T) {
	a := New("admin", "p", "apikey", "k", time.Hour)
	tok, _ := a.Login("admin", "p")
	require.True(t, a.Valid(tok))      // 会话令牌
	require.True(t, a.Valid("apikey")) // API Token 同时有效
}
