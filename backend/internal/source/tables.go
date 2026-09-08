package source

// 以下常量均已连生产库实测确认，写死不再走环境变量。
const (
	// KKUDTable kkud 库代理用户表（PK id，daili 列=代理）。
	KKUDTable = "user568531942"
	// KKUDVIPColumn VIP 列（char(255)）：空串=非 VIP，'1'=VIP。
	KKUDVIPColumn = "vip"
	// FenxUsersTable fenx_site 用户表（PK uid）。
	FenxUsersTable = "zyads_users"
	// FenxLoginLogTable fenx_site 登录日志表（按 username 关联，status=1 成功）。
	FenxLoginLogTable = "zyads_log_login"
	// FenxLevelTable fenx_site 会员等级表（levelid PK / levelname）。
	FenxLevelTable = "zyads_level"
	// FenxDisabledStatus 禁用状态值（zyads_users.status：0=待审核 1=待激活 2=正常 4=禁用）。
	FenxDisabledStatus = 4
	// FenxNormalStatus 正常状态值（登录要求 status==2）。
	FenxNormalStatus = 2
)
