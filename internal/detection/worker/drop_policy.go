package worker

import v1 "github.com/argusxdr/argus/gen/go/argus/v1"

// SeverityInt maps a proto Severity enum to an integer 0..5.
// 0=unspecified, 1=info, 2=low, 3=medium, 4=high, 5=critical
func SeverityInt(s v1.Severity) int {
	switch s {
	case v1.Severity_INFO:
		return 1
	case v1.Severity_LOW:
		return 2
	case v1.Severity_MEDIUM:
		return 3
	case v1.Severity_HIGH:
		return 4
	case v1.Severity_CRITICAL:
		return 5
	default:
		return 0
	}
}

// SeverityLabel returns the metric label string for a severity integer.
func SeverityLabel(sevInt int) string {
	switch sevInt {
	case 1:
		return "info"
	case 2:
		return "low"
	case 3:
		return "medium"
	case 4:
		return "high"
	case 5:
		return "critical"
	default:
		return "unspecified"
	}
}
