package ipfraud

import (
	"fmt"
	"strconv"
	"strings"
)

func stringValue(value map[string]any, paths ...string) string {
	for _, path := range paths {
		if raw, ok := pathValue(value, path); ok {
			switch typed := raw.(type) {
			case string:
				if out := strings.TrimSpace(typed); out != "" {
					return out
				}
			case float64:
				return strconv.FormatFloat(typed, 'f', -1, 64)
			case bool:
				return strconv.FormatBool(typed)
			}
		}
	}
	return ""
}

func boolValue(value map[string]any, paths ...string) bool {
	for _, path := range paths {
		if raw, ok := pathValue(value, path); ok {
			switch typed := raw.(type) {
			case bool:
				return typed
			case string:
				switch strings.ToLower(strings.TrimSpace(typed)) {
				case "1", "true", "yes", "y":
					return true
				case "0", "false", "no", "n":
					return false
				}
			case float64:
				return typed != 0
			}
		}
	}
	return false
}

func floatValue(value map[string]any, paths ...string) (float64, bool) {
	for _, path := range paths {
		if raw, ok := pathValue(value, path); ok {
			switch typed := raw.(type) {
			case float64:
				return typed, true
			case string:
				parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
				if err == nil {
					return parsed, true
				}
			}
		}
	}
	return 0, false
}

func pathValue(value map[string]any, path string) (any, bool) {
	current := any(value)
	for _, part := range strings.Split(path, ".") {
		item, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = item[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func compactLower(values ...string) string {
	for _, value := range values {
		if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
			return value
		}
	}
	return ""
}

func asnValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToUpper(value), "AS") {
		return strings.ToUpper(value)
	}
	return fmt.Sprintf("AS%s", value)
}
