package compliance

import (
	"igaming-proxy/internal/config"
)

// LicenseValidator validates provider licenses against required licenses.
type LicenseValidator struct {
	providerLicenses map[string][]string
}

// NewLicenseValidator creates a LicenseValidator from provider configuration.
func NewLicenseValidator(providersConfig config.ProvidersConfig) *LicenseValidator {
	licenses := make(map[string][]string, len(providersConfig.Providers))
	for _, p := range providersConfig.Providers {
		licenses[p.ID] = p.Licenses
	}
	return &LicenseValidator{providerLicenses: licenses}
}

// ValidateLicense checks whether providerID holds the requiredLicense.
func (v *LicenseValidator) ValidateLicense(providerID, requiredLicense string) bool {
	licenses, ok := v.providerLicenses[providerID]
	if !ok {
		return false
	}
	for _, l := range licenses {
		if l == requiredLicense {
			return true
		}
	}
	return false
}

// GetProviderLicenses returns all licenses held by the given provider.
func (v *LicenseValidator) GetProviderLicenses(providerID string) []string {
	return v.providerLicenses[providerID]
}

// IsProviderLicensedForRegion checks whether a provider is licensed for the given region name.
func (v *LicenseValidator) IsProviderLicensedForRegion(providerID, region string) bool {
	regionLicenseMap := map[string]string{
		"EU":    "MGA",
		"UK":    "UKGC",
		"LATAM": "LOCAL",
		"APAC":  "PAGCOR",
	}
	requiredLicense, ok := regionLicenseMap[region]
	if !ok {
		return false
	}
	return v.ValidateLicense(providerID, requiredLicense)
}
