package models

import "time"

type PlatformUser struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:80" json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `gorm:"size:30;index" json:"role"`
	AgentID      *uint     `json:"agent_id,omitempty"`
	Status       string    `gorm:"size:20;default:active" json:"status"`
	LastLoginIP  string    `gorm:"size:64" json:"last_login_ip"`
	CreatedAt    time.Time `json:"created_at"`
}

type Agent struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	DisplayName  string    `gorm:"size:120" json:"display_name"`
	SourceValues string    `gorm:"type:text" json:"source_values"`
	Enabled      bool      `gorm:"index" json:"enabled"`
	PolicyJSON   *string   `gorm:"type:jsonb" json:"policy_json"`
	CreatedAt    time.Time `json:"created_at"`
}

type MonitorTask struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	AgentID     uint       `gorm:"index" json:"agent_id"`
	IntervalSec int        `json:"interval_sec"`
	Enabled     bool       `gorm:"index" json:"enabled"`
	NextRunAt   *time.Time `json:"next_run_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type AuditEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ActorID   *uint     `json:"actor_id,omitempty"`
	Action    string    `gorm:"size:80;index" json:"action"`
	Target    string    `gorm:"size:160" json:"target"`
	Detail    string    `gorm:"type:text" json:"detail,omitempty"`
	RequestID string    `gorm:"size:80;index" json:"request_id"`
	CreatedAt time.Time `json:"created_at"`
}

type IPClaim struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   uint      `gorm:"uniqueIndex:ux_ip_claim;index" json:"agent_id"`
	IP        string    `gorm:"size:64;uniqueIndex:ux_ip_claim" json:"ip"`
	Action    string    `gorm:"size:20" json:"action"`
	AccountID string    `gorm:"size:120;index" json:"account_id"`
	CreatedAt time.Time `json:"created_at"`
}

type SyncRun struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	TaskID     uint       `gorm:"index" json:"task_id"`
	Status     string     `gorm:"size:20;index" json:"status"`
	RowsRead   int        `json:"rows_read"`
	RowsSaved  int        `json:"rows_saved"`
	Error      string     `gorm:"type:text" json:"error,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type KKUDSnapshot struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	TaskID           uint      `gorm:"index" json:"task_id"`
	SourcePK         string    `gorm:"size:120;index" json:"source_pk"`
	AgentValue       string    `gorm:"size:160;index" json:"agent_value"`
	Mobile           string    `gorm:"size:40;index" json:"mobile"`
	QQ               string    `gorm:"size:40;index" json:"qq"`
	Email            string    `gorm:"size:160;index" json:"email"`
	NormalizedMobile string    `gorm:"size:40;index" json:"normalized_mobile"`
	NormalizedQQ     string    `gorm:"size:40;index" json:"normalized_qq"`
	NormalizedEmail  string    `gorm:"size:160;index" json:"normalized_email"`
	DataJSON         *string   `gorm:"type:jsonb" json:"-"`
	DataHash         string    `gorm:"size:64;index" json:"data_hash"`
	CapturedAt       time.Time `json:"captured_at"`
}

type UserMatch struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	RunID          uint      `gorm:"index" json:"run_id"`
	KKUDSnapshotID uint      `gorm:"index" json:"kkud_snapshot_id"`
	FenxUserID     string    `gorm:"size:120;index" json:"fenx_user_id"`
	Confidence     float64   `json:"confidence"`
	MatchedFields  string    `gorm:"size:120" json:"matched_fields"`
	State          string    `gorm:"size:20;index" json:"state"`
	CreatedAt      time.Time `json:"created_at"`
}

type IPConflict struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	AgentID      uint       `gorm:"uniqueIndex:ux_ip_conflict;index" json:"agent_id"`
	IP           string     `gorm:"size:64;uniqueIndex:ux_ip_conflict" json:"ip"`
	ConflictType string     `gorm:"size:20;uniqueIndex:ux_ip_conflict" json:"conflict_type"`
	Risk         string     `gorm:"size:20;index" json:"risk"`
	Status       string     `gorm:"size:20;index;default:open" json:"status"`
	WinnerUID    string     `gorm:"size:120" json:"winner_uid"`
	MemberCount  int        `json:"member_count"`
	FirstSeenAt  time.Time  `json:"first_seen_at"`
	LastSeenAt   time.Time  `json:"last_seen_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ConflictMember struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ConflictID   uint      `gorm:"index" json:"conflict_id"`
	FenxUID      string    `gorm:"size:120;index" json:"fenx_uid"`
	Username     string    `gorm:"size:120" json:"username"`
	RegIP        string    `gorm:"size:64" json:"reg_ip"`
	LoginIP      string    `gorm:"size:64" json:"login_ip"`
	MatchState   string    `gorm:"size:20" json:"match_state"`
	EvidenceJSON *string   `gorm:"type:jsonb" json:"evidence_json"`
	IsWinner     bool      `json:"is_winner"`
	CreatedAt    time.Time `json:"created_at"`
}

type ActionJob struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	ConflictID     uint       `gorm:"index" json:"conflict_id"`
	Action         string     `gorm:"size:20;index" json:"action"`
	Status         string     `gorm:"size:20;index;default:pending" json:"status"`
	TargetUID      string     `gorm:"size:120;index" json:"target_uid"`
	BeforeJSON     *string    `gorm:"type:jsonb" json:"before_json"`
	AfterJSON      *string    `gorm:"type:jsonb" json:"after_json"`
	IdempotencyKey string     `gorm:"size:160;uniqueIndex" json:"idempotency_key"`
	OperatorID     *uint      `json:"operator_id,omitempty"`
	Error          string     `gorm:"type:text" json:"error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ExecutedAt     *time.Time `json:"executed_at,omitempty"`
}

type Allowlist struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	AgentID   uint       `gorm:"index" json:"agent_id"`
	FenxUID   string     `gorm:"size:120;index" json:"fenx_uid"`
	IP        string     `gorm:"size:64;index" json:"ip"`
	Reason    string     `gorm:"size:200" json:"reason"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type SysConfig struct {
	Key       string    `gorm:"primaryKey;size:80" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy *uint     `json:"updated_by,omitempty"`
}

// RefreshToken 可撤销的刷新令牌；库中只存 sha256 哈希，不存原文。
type RefreshToken struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `gorm:"index" json:"user_id"`
	TokenHash  string     `gorm:"size:64;uniqueIndex" json:"-"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}
