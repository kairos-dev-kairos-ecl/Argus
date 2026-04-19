package worker

import (
	"testing"

	v1 "github.com/argusxdr/argus/gen/go/argus/v1"
	"github.com/stretchr/testify/assert"
)

func TestSeverityInt_AllEnums(t *testing.T) {
	cases := []struct {
		sev      v1.Severity
		expected int
	}{
		{v1.Severity_INFO, 1},
		{v1.Severity_LOW, 2},
		{v1.Severity_MEDIUM, 3},
		{v1.Severity_HIGH, 4},
		{v1.Severity_CRITICAL, 5},
		{v1.Severity_SEVERITY_UNSPECIFIED, 0},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.expected, SeverityInt(tc.sev), "severity %v", tc.sev)
	}
}

func TestSeverityLabel(t *testing.T) {
	assert.Equal(t, "info", SeverityLabel(1))
	assert.Equal(t, "low", SeverityLabel(2))
	assert.Equal(t, "medium", SeverityLabel(3))
	assert.Equal(t, "high", SeverityLabel(4))
	assert.Equal(t, "critical", SeverityLabel(5))
	assert.Equal(t, "unspecified", SeverityLabel(0))
}
