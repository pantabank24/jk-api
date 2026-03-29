package useragent

import "strings"

// ParseDevice returns a human-readable device/browser string from a User-Agent header.
func ParseDevice(ua string) string {
	lower := strings.ToLower(ua)

	os := parseOS(lower)
	browser := parseBrowser(lower)

	return browser + " on " + os
}

func parseOS(ua string) string {
	switch {
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "iphone"):
		return "iPhone"
	case strings.Contains(ua, "ipad"):
		return "iPad"
	case strings.Contains(ua, "windows nt"):
		return "Windows"
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os x"):
		return "macOS"
	case strings.Contains(ua, "linux"):
		return "Linux"
	default:
		return "Unknown"
	}
}

func parseBrowser(ua string) string {
	switch {
	case strings.Contains(ua, "edg/") || strings.Contains(ua, "edge/"):
		return "Edge"
	case strings.Contains(ua, "opr/") || strings.Contains(ua, "opera"):
		return "Opera"
	case strings.Contains(ua, "chrome") && !strings.Contains(ua, "chromium"):
		return "Chrome"
	case strings.Contains(ua, "firefox"):
		return "Firefox"
	case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
		return "Safari"
	case strings.Contains(ua, "postmanruntime"):
		return "Postman"
	case strings.Contains(ua, "curl"):
		return "curl"
	default:
		return "Unknown"
	}
}
