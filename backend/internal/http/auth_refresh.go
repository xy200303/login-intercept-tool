package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"fenx/backend/internal/models"
)

// issueRefreshToken 生成 32 字节随机 refresh token；库中只存 sha256 哈希，原文仅此一次返回。
func (a *API) issueRefreshToken(userID uint) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	row := models.RefreshToken{
		UserID:    userID,
		TokenHash: refreshTokenHash(token),
		ExpiresAt: time.Now().UTC().Add(a.cfg.RefreshTokenTTL),
		CreatedAt: time.Now().UTC(),
	}
	if err := a.db.Create(&row).Error; err != nil {
		return "", err
	}
	return token, nil
}

func refreshTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// tokenResponse 登录/刷新共用的令牌响应字段。
func (a *API) tokenResponse(accessToken, refreshToken string, userID uint, username, role string, agentID *uint) map[string]any {
	out := map[string]any{
		"access_token": accessToken,
		"expires_in":   int(a.cfg.AccessTokenTTL.Seconds()),
		"token_type":   "Bearer",
		"user":         map[string]any{"id": userID, "username": username, "role": role, "agent_id": agentID},
	}
	if refreshToken != "" {
		out["refresh_token"] = refreshToken
	}
	return out
}

// refresh 轮换 refresh token：校验（哈希查找+未撤销+未过期）→ 旧 token 撤销 → 签发新令牌对。
// 无效/过期/已撤销统一 401 invalid_refresh_token，不区分原因（防爆破枚举）。
func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	invalid := func() {
		write(w, 401, map[string]string{"message": "invalid_refresh_token"})
	}
	if a.db == nil {
		write(w, 503, map[string]string{"message": "数据库不可用"})
		return
	}
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.RefreshToken == "" {
		invalid()
		return
	}
	var stored models.RefreshToken
	if err := a.db.Where("token_hash = ?", refreshTokenHash(in.RefreshToken)).First(&stored).Error; err != nil {
		invalid()
		return
	}
	now := time.Now().UTC()
	if stored.RevokedAt != nil || now.After(stored.ExpiresAt) {
		invalid()
		return
	}
	var user models.PlatformUser
	if err := a.db.First(&user, stored.UserID).Error; err != nil || user.Status != "active" {
		invalid()
		return
	}
	stored.RevokedAt = &now
	stored.LastUsedAt = &now
	if err := a.db.Save(&stored).Error; err != nil {
		write(w, 500, map[string]string{"message": "令牌轮换失败"})
		return
	}
	newRefresh, err := a.issueRefreshToken(user.ID)
	if err != nil {
		write(w, 500, map[string]string{"message": "令牌签发失败"})
		return
	}
	access, err := a.jwt.Issue(user.ID, user.Username, user.Role)
	if err != nil {
		write(w, 500, map[string]string{"message": "token error"})
		return
	}
	write(w, 200, a.tokenResponse(access, newRefresh, user.ID, user.Username, user.Role, user.AgentID))
}

// logout 撤销指定 refresh token（幂等：不存在/已撤销也返回成功）。
func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if a.db != nil && in.RefreshToken != "" {
		now := time.Now().UTC()
		a.db.Model(&models.RefreshToken{}).
			Where("token_hash = ? AND revoked_at IS NULL", refreshTokenHash(in.RefreshToken)).
			Update("revoked_at", now)
	}
	write(w, 200, map[string]any{"logged_out": true})
}

// revokeUserRefreshTokens 撤销某用户全部未撤销的 refresh token（改密后强制重登）。
func (a *API) revokeUserRefreshTokens(userID uint) {
	if a.db == nil {
		return
	}
	now := time.Now().UTC()
	a.db.Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now)
}
