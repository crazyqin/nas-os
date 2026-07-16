package kerberos

import "time"

// KerberosConfig Kerberos 配置.
type KerberosConfig struct {
	Realm          string        `json:"realm"`
	KDCHost        string        `json:"kdc_host"`
	KDCPort        int           `json:"kdc_port"`
	AdminHost      string        `json:"admin_host"`
	AdminPort      int           `json:"admin_port"`
	DefaultDomain  string        `json:"default_domain"`
	TicketLifetime time.Duration `json:"ticket_lifetime"`
	RenewLifetime  time.Duration `json:"renew_lifetime"`
	MaxRetries     int           `json:"max_retries"`
	CacheSize      int           `json:"cache_size"`
	KeytabPath     string        `json:"keytab_path"`
	EnableUDP      bool          `json:"enable_udp"`
	EnableTCP      bool          `json:"enable_tcp"`
	PreauthType    string        `json:"preauth_type"`
}

// Realm Kerberos Realm.
type Realm struct {
	Name        string    `json:"name"`
	KDCHost     string    `json:"kdc_host"`
	KDCPort     int       `json:"kdc_port"`
	AdminHost   string    `json:"admin_host"`
	AdminPort   int       `json:"admin_port"`
	MasterKey   string    `json:"-"` // 不序列化
	Domain      string    `json:"domain"`
	KpasswdHost string    `json:"kpasswd_host"`
	CreatedAt   time.Time `json:"created_at"`
}

// Principal Kerberos Principal.
type Principal struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Realm        string            `json:"realm"`
	Type         PrincipalType     `json:"type"`
	Attributes   map[string]string `json:"attributes"`
	MaxLife      time.Duration     `json:"max_life"`
	MaxRenew     time.Duration     `json:"max_renew"`
	ExpiresAt    time.Time         `json:"expires_at"`
	Disabled     bool              `json:"disabled"`
	LockedOut    bool              `json:"locked_out"`
	FailedLogins int               `json:"failed_logins"`
	LastLogin    time.Time         `json:"last_login"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// PrincipalType Principal 类型.
type PrincipalType string

const (
	PrincipalTypeUser    PrincipalType = "user"
	PrincipalTypeService PrincipalType = "service"
	PrincipalTypeHost    PrincipalType = "host"
	PrincipalTypeAdmin   PrincipalType = "admin"
)

// Keytab Kerberos Keytab.
type Keytab struct {
	ID          string    `json:"id"`
	PrincipalID string    `json:"principal_id"`
	Version     int       `json:"version"`
	Enctype     string    `json:"enctype"`
	Key         string    `json:"-"` // 不序列化
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// Ticket Kerberos 票据.
type Ticket struct {
	ID          string    `json:"id"`
	PrincipalID string    `json:"principal_id"`
	Service     string    `json:"service"`
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Renewable   bool      `json:"renewable"`
	RenewedAt   time.Time `json:"renewed_at"`
	Forwardable bool      `json:"forwardable"`
	Proxiable   bool      `json:"proxiable"`
	Addresses   []string  `json:"addresses"`
}

// KerberosStats 统计信息.
type KerberosStats struct {
	Realm           string `json:"realm"`
	TotalPrincipals int    `json:"total_principals"`
	TotalKeytabs    int    `json:"total_keytabs"`
	TotalTickets    int    `json:"total_tickets"`
	ActiveTickets   int    `json:"active_tickets"`
	ExpiredTickets  int    `json:"expired_tickets"`
}

// AuthRequest 认证请求.
type AuthRequest struct {
	Principal   string `json:"principal"`
	Password    string `json:"password"`
	KeytabID    string `json:"keytab_id"`
	Service     string `json:"service"`
	Renewable   bool   `json:"renewable"`
	Forwardable bool   `json:"forwardable"`
}

// AuthResponse 认证响应.
type AuthResponse struct {
	Success   bool      `json:"success"`
	TicketID  string    `json:"ticket_id"`
	ExpiresAt time.Time `json:"expires_at"`
	ErrorMsg  string    `json:"error_msg,omitempty"`
}
