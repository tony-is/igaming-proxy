package compliance

import (
	"igaming-proxy/internal/config"
	"igaming-proxy/internal/geoip"
)

// RoutingDecision holds the result of a compliance and routing evaluation.
type RoutingDecision struct {
	Allowed          bool
	ProviderID       string
	ProviderBaseURL  string
	Region           string
	License          string
	BlockReason      string
	Restrictions     map[string]interface{}
	ComplianceFlags  []string
}

// Engine evaluates routing decisions based on geo location and compliance rules.
type Engine struct {
	routingRules    config.RoutingRules
	providers       map[string]config.Provider
	complianceRules config.ComplianceRules
	countryRegion   map[string]*config.Region
	licenseValidator *LicenseValidator
}

// NewEngine creates a new compliance Engine with pre-computed lookup maps.
func NewEngine(routingRules config.RoutingRules, providersConfig config.ProvidersConfig, complianceRules config.ComplianceRules) *Engine {
	providerMap := make(map[string]config.Provider, len(providersConfig.Providers))
	for _, p := range providersConfig.Providers {
		providerMap[p.ID] = p
	}

	countryRegion := make(map[string]*config.Region)
	for i := range routingRules.Regions {
		region := &routingRules.Regions[i]
		for _, cc := range region.Countries {
			countryRegion[cc] = region
		}
	}

	return &Engine{
		routingRules:     routingRules,
		providers:        providerMap,
		complianceRules:  complianceRules,
		countryRegion:    countryRegion,
		licenseValidator: NewLicenseValidator(providersConfig),
	}
}

// Evaluate determines the routing decision for a request.
func (e *Engine) Evaluate(geo *geoip.GeoResult, productType, preferredProvider string) *RoutingDecision {
	region, ok := e.countryRegion[geo.CountryCode]
	if !ok {
		return &RoutingDecision{
			Allowed:     false,
			BlockReason: "unknown region",
			Region:      "UNKNOWN",
		}
	}

	if region.Action == "block" {
		return &RoutingDecision{
			Allowed:     false,
			Region:      region.Name,
			BlockReason: region.BlockMessage,
		}
	}

	// Select provider
	providerID := region.DefaultProvider
	if preferredProvider != "" {
		for _, allowed := range region.AllowedProviders {
			if allowed == preferredProvider {
				providerID = preferredProvider
				break
			}
		}
	}

	provider, exists := e.providers[providerID]
	if !exists {
		return &RoutingDecision{
			Allowed:     false,
			BlockReason: "provider not found: " + providerID,
			Region:      region.Name,
		}
	}

	flags := e.gatherComplianceFlags(geo.CountryCode, region)

	return &RoutingDecision{
		Allowed:         true,
		ProviderID:      providerID,
		ProviderBaseURL: provider.BaseURL,
		Region:          region.Name,
		License:         region.License,
		Restrictions:    region.Restrictions,
		ComplianceFlags: flags,
	}
}

// gatherComplianceFlags collects applicable compliance flags for a country/region.
func (e *Engine) gatherComplianceFlags(countryCode string, region *config.Region) []string {
	var flags []string

	if e.complianceRules.Global.KYCRequired {
		flags = append(flags, "KYC_REQUIRED")
	}
	if e.complianceRules.Global.AMLCheck {
		flags = append(flags, "AML_CHECK")
	}

	override, hasOverride := e.complianceRules.JurisdictionOverrides[region.License]
	if hasOverride {
		if override.EnhancedKYC {
			flags = append(flags, "ENHANCED_KYC")
		}
		if override.SourceOfFundsCheck {
			flags = append(flags, "SOURCE_OF_FUNDS_CHECK")
		}
		if override.GamstopCheck {
			flags = append(flags, "GAMSTOP_CHECK")
		}
	}

	if r, ok := region.Restrictions["self_exclusion_check"]; ok {
		if b, ok := r.(bool); ok && b {
			flags = append(flags, "SELF_EXCLUSION_CHECK")
		}
	}

	return flags
}

// ValidateProviderLicense checks whether the provider holds the required license.
func (e *Engine) ValidateProviderLicense(providerID, requiredLicense string) bool {
	return e.licenseValidator.ValidateLicense(providerID, requiredLicense)
}

// GetBlockedPaymentMethods returns blocked payment methods for the given country.
func (e *Engine) GetBlockedPaymentMethods(countryCode string) []string {
	var methods []string
	for _, blocked := range e.complianceRules.BlockedPaymentMethods {
		for _, cc := range blocked.Countries {
			if cc == countryCode {
				methods = append(methods, blocked.Method)
				break
			}
		}
	}
	return methods
}
