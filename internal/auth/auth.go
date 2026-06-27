// Package auth 提供账户密码登录与签名会话令牌（无外部依赖，HMAC-SHA256）。
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Authenticator 负责登录校验与会话令牌签发/验证。
type Authenticator struct {
	user     string        // 登录用户名（空=不启用账密登录）
	pass     string        // 登录密码（明文，置于 k8s Secret）
	apiToken string        // 静态 API Token（空=不启用；供脚本调用）
	secret   []byte        // 会话令牌 HMAC 签名密钥
	ttl      time.Duration // 会话有效期
}

// New 构造 Authenticator。secret 为空时随机生成（重启后旧会话失效）。
func New(user, pass, apiToken, secret string, ttl time.Duration) *Authenticator {
	s := []byte(secret)
	if len(s) == 0 {
		s = make([]byte, 32)
		_, _ = rand.Read(s)
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Authenticator{user: user, pass: pass, apiToken: apiToken, secret: s, ttl: ttl}
}

// Enabled 是否启用了任何鉴权（账密或 API Token）。
func (a *Authenticator) Enabled() bool { return a.user != "" || a.apiToken != "" }

// LoginEnabled 是否启用了账号密码登录。
func (a *Authenticator) LoginEnabled() bool { return a.user != "" && a.pass != "" }

// Login 校验账号密码，成功返回会话令牌。
func (a *Authenticator) Login(user, pass string) (string, bool) {
	if !a.LoginEnabled() {
		return "", false
	}
	if ctEq(user, a.user) && ctEq(pass, a.pass) {
		return a.issue(user), true
	}
	return "", false
}

// Valid 校验一个凭证：有效会话令牌，或匹配的静态 API Token。
func (a *Authenticator) Valid(cred string) bool {
	if !a.Enabled() {
		return true
	}
	if cred == "" {
		return false
	}
	if a.apiToken != "" && ctEq(cred, a.apiToken) {
		return true
	}
	return a.validSession(cred)
}

func (a *Authenticator) issue(user string) string {
	payload := fmt.Sprintf("%s|%d", user, time.Now().Add(a.ttl).Unix())
	return enc(payload) + "." + enc(string(a.sign(payload)))
}

func (a *Authenticator) sign(payload string) []byte {
	m := hmac.New(sha256.New, a.secret)
	m.Write([]byte(payload))
	return m.Sum(nil)
}

func (a *Authenticator) validSession(token string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	payload, err := dec(parts[0])
	if err != nil {
		return false
	}
	sig, err := dec(parts[1])
	if err != nil {
		return false
	}
	if !hmac.Equal([]byte(sig), a.sign(payload)) {
		return false
	}
	seg := strings.SplitN(payload, "|", 2)
	if len(seg) != 2 {
		return false
	}
	exp, err := strconv.ParseInt(seg[1], 10, 64)
	if err != nil {
		return false
	}
	return time.Now().Unix() < exp
}

func ctEq(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

func enc(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func dec(s string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	return string(b), err
}
