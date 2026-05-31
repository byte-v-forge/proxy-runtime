package accountproxy

import (
	"strings"

	"github.com/biter777/countries"
	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/geox"
)

func init() {
	RegisterDefinition(Definition{
		ProviderID:               ProviderB2Proxy,
		DisplayName:              "B2Proxy",
		DefaultProtocol:          "socks5",
		Protocols:                []string{"http", "socks5"},
		UsernameParameterSession: true,
		RuntimeGeoTargeting:      true,
		BuildUsername:            b2proxyUsername,
		GenerateSessionID:        numericSessionID,
	})
}

func b2proxyUsername(base string, policy *proxyruntimev1.ProxySessionPolicy, sessionID string) string {
	countryCode, stateCode := b2proxyGeo(policy)
	return dashUsername(base,
		"zone", "custom",
		"region", countryCode,
		"st", b2proxyStateName(countryCode, stateCode),
		"session", sessionID,
		"sessTime", stickyMinutesString(policy),
		"sessAuto", "1",
	)
}

func b2proxyGeo(policy *proxyruntimev1.ProxySessionPolicy) (string, string) {
	if policy == nil {
		return "", ""
	}
	region := strings.ToUpper(strings.TrimSpace(policy.GetRegion()))
	countryCode := geox.NormalizeCountryAlpha2(region)
	stateCode := strings.TrimSpace(policy.GetState())
	if prefix, suffix, ok := strings.Cut(region, "-"); ok {
		if countryCode == "" {
			countryCode = geox.NormalizeCountryAlpha2(prefix)
		}
		if stateCode == "" {
			stateCode = suffix
		}
	}
	return countryCode, stateCode
}

func b2proxyStateName(countryCode string, stateCode string) string {
	countryCode = geox.NormalizeCountryAlpha2(countryCode)
	stateCode = strings.TrimSpace(stateCode)
	switch countryCode {
	case "SG":
		return "Singapore"
	case "HK":
		return "Hong Kong"
	}
	if name := subdivisionEnglishName(countryCode, stateCode); name != "" {
		return name
	}
	return titleRegionName(stateCode)
}

func subdivisionEnglishName(countryCode string, value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	candidates := []string{value}
	if countryCode != "" && !strings.Contains(value, "-") {
		candidates = append([]string{countryCode + "-" + value}, candidates...)
	}
	for _, candidate := range candidates {
		subdivision := countries.SubdivisionCode(candidate)
		if !subdivision.IsValid() {
			continue
		}
		if countryCode != "" && !strings.EqualFold(subdivision.Country().Alpha2(), countryCode) {
			continue
		}
		return subdivision.String()
	}
	return ""
}

func titleRegionName(value string) string {
	value = strings.NewReplacer("_", " ", "-", " ").Replace(strings.TrimSpace(value))
	fields := strings.Fields(value)
	for index, field := range fields {
		lower := strings.ToLower(field)
		if lower == "" {
			continue
		}
		fields[index] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(fields, " ")
}
