package config

// Config is the top-level configuration struct.
type Config struct {
	Server       ServerConfig  `yaml:"server"`
	GeoIP        GeoIPConfig   `yaml:"geoip"`
	Auth         AuthConfig    `yaml:"auth"`
	RateLimit    RateLimitConfig `yaml:"rate_limit"`
	Audit        AuditConfig   `yaml:"audit"`
	RoutingRules RoutingRules  `yaml:"-"`
	Providers    ProvidersConfig `yaml:"-"`
	Compliance   ComplianceRules `yaml:"-"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port            int    `yaml:"port"`
	ShutdownTimeout int    `yaml:"shutdown_timeout_seconds"`
}

// GeoIPConfig holds GeoIP database settings.
type GeoIPConfig struct {
	DatabasePath string `yaml:"database_path"`
	CacheTTLMin  int    `yaml:"cache_ttl_minutes"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret  string `yaml:"jwt_secret"`
	APIKeyHeader string `yaml:"api_key_header"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	RequestsPerSecond float64 `yaml:"requests_per_second"`
	Burst             int     `yaml:"burst"`
}

// AuditConfig holds audit logging settings.
type AuditConfig struct {
	OutputType string `yaml:"output_type"` // stdout, file, kafka
	FilePath   string `yaml:"file_path"`
	KafkaTopic string `yaml:"kafka_topic"`
}

// RoutingRules holds the region-based routing configuration.
type RoutingRules struct {
	Regions []Region `yaml:"regions"`
}

// Region defines routing behavior for a set of countries.
type Region struct {
	Name             string            `yaml:"name"`
	Countries        []string          `yaml:"countries"`
	DefaultProvider  string            `yaml:"default_provider"`
	License          string            `yaml:"license"`
	AllowedProviders []string          `yaml:"allowed_providers"`
	Action           string            `yaml:"action"`
	BlockMessage     string            `yaml:"block_message"`
	Restrictions     map[string]interface{} `yaml:"restrictions"`
}

// ProvidersConfig wraps the list of providers.
type ProvidersConfig struct {
	Providers []Provider `yaml:"providers"`
}

// Provider defines a third-party game provider.
type Provider struct {
	ID             string         `yaml:"id"`
	Name           string         `yaml:"name"`
	Licenses       []string       `yaml:"licenses"`
	GameTypes      []string       `yaml:"game_types"`
	BaseURL        string         `yaml:"base_url"`
	HealthCheck    string         `yaml:"health_check"`
	TimeoutMS      int            `yaml:"timeout_ms"`
	RetryCount     int            `yaml:"retry_count"`
	Auth           ProviderAuth   `yaml:"auth"`
	CircuitBreaker CircuitBreaker `yaml:"circuit_breaker"`
}

// ProviderAuth defines how to authenticate with a provider.
type ProviderAuth struct {
	Type   string `yaml:"type"` // api_key, bearer_token, hmac
	Header string `yaml:"header"`
	Value  string `yaml:"value"` // populated from env
}

// CircuitBreaker holds circuit breaker settings for a provider.
type CircuitBreaker struct {
	Threshold      int `yaml:"threshold"`
	TimeoutSeconds int `yaml:"timeout_seconds"`
}

// ComplianceRules holds all compliance configuration.
type ComplianceRules struct {
	Global                  GlobalCompliance               `yaml:"global"`
	JurisdictionOverrides   map[string]JurisdictionOverride `yaml:"jurisdiction_overrides"`
	BlockedPaymentMethods   []BlockedPaymentMethod         `yaml:"blocked_payment_methods"`
}

// GlobalCompliance defines default compliance requirements.
type GlobalCompliance struct {
	MinAge             int  `yaml:"min_age"`
	KYCRequired        bool `yaml:"kyc_required"`
	AMLCheck           bool `yaml:"aml_check"`
	ResponsibleGaming  bool `yaml:"responsible_gaming"`
}

// JurisdictionOverride defines compliance overrides per jurisdiction.
type JurisdictionOverride struct {
	MinAge                  int    `yaml:"min_age"`
	EnhancedKYC             bool   `yaml:"enhanced_kyc"`
	SourceOfFundsCheck      bool   `yaml:"source_of_funds_check"`
	AffordabilityAssessment bool   `yaml:"affordability_assessment"`
	GamstopCheck            bool   `yaml:"gamstop_check"`
	Marketing               string `yaml:"marketing"`
	AuditRetentionYears     int    `yaml:"audit_retention_years"`
	SelfExclusionRegistry   bool   `yaml:"self_exclusion_registry"`
	RealityCheck            bool   `yaml:"reality_check"`
	LocalBankVerification   bool   `yaml:"local_bank_verification"`
}

// BlockedPaymentMethod defines payment methods blocked in specific countries.
type BlockedPaymentMethod struct {
	Method    string   `yaml:"method"`
	Countries []string `yaml:"countries"`
}
