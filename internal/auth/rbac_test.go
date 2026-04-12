package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPermissionMatrix(t *testing.T) {
	pc := NewPermissionChecker()

	// Admin should have all permissions
	assert.True(t, pc.HasPermission(RoleAdmin, PermUsersCreate))
	assert.True(t, pc.HasPermission(RoleAdmin, PermRulesCreate))
	assert.True(t, pc.HasPermission(RoleAdmin, PermConfigUpdate))
	assert.True(t, pc.HasPermission(RoleAdmin, PermAuditRead))

	// Analyst should have read/acknowledge but not create
	assert.False(t, pc.HasPermission(RoleAnalyst, PermUsersCreate))
	assert.False(t, pc.HasPermission(RoleAnalyst, PermRulesCreate))
	assert.True(t, pc.HasPermission(RoleAnalyst, PermAlertsRead))
	assert.True(t, pc.HasPermission(RoleAnalyst, PermAlertsAcknowledge))
	assert.True(t, pc.HasPermission(RoleAnalyst, PermAuditRead))

	// Viewer should only have read
	assert.False(t, pc.HasPermission(RoleViewer, PermUsersCreate))
	assert.False(t, pc.HasPermission(RoleViewer, PermRulesCreate))
	assert.False(t, pc.HasPermission(RoleViewer, PermAuditRead)) // Viewers can't see audit
	assert.True(t, pc.HasPermission(RoleViewer, PermAlertsRead))
	assert.True(t, pc.HasPermission(RoleViewer, PermRulesRead))
}

func TestGetPermissionsForRole(t *testing.T) {
	pc := NewPermissionChecker()

	adminPerms := pc.GetPermissionsForRole(RoleAdmin)
	assert.Greater(t, len(adminPerms), 5)
	assert.Contains(t, adminPerms, PermUsersCreate)

	analystPerms := pc.GetPermissionsForRole(RoleAnalyst)
	assert.Greater(t, len(analystPerms), 0)
	assert.Less(t, len(analystPerms), len(adminPerms))

	viewerPerms := pc.GetPermissionsForRole(RoleViewer)
	assert.Greater(t, len(viewerPerms), 0)
	assert.Less(t, len(viewerPerms), len(analystPerms))

	// Unknown role should return empty slice
	unknownPerms := pc.GetPermissionsForRole("unknown")
	assert.Equal(t, 0, len(unknownPerms))
}

func TestRoleManagement(t *testing.T) {
	// Admin can manage all roles
	assert.True(t, CanManageRole(RoleAdmin, RoleAdmin))
	assert.True(t, CanManageRole(RoleAdmin, RoleAnalyst))
	assert.True(t, CanManageRole(RoleAdmin, RoleViewer))

	// Analyst cannot manage anyone
	assert.False(t, CanManageRole(RoleAnalyst, RoleAdmin))
	assert.False(t, CanManageRole(RoleAnalyst, RoleAnalyst))
	assert.False(t, CanManageRole(RoleAnalyst, RoleViewer))

	// Viewer cannot manage anyone
	assert.False(t, CanManageRole(RoleViewer, RoleAdmin))
	assert.False(t, CanManageRole(RoleViewer, RoleAnalyst))
	assert.False(t, CanManageRole(RoleViewer, RoleViewer))
}

func TestPermissionDescriptions(t *testing.T) {
	desc := GetPermissionDescription(PermUsersCreate)
	assert.NotEmpty(t, desc)
	assert.Equal(t, "Create new users", desc)

	unknownDesc := GetPermissionDescription("unknown:permission")
	assert.Equal(t, "Unknown permission", unknownDesc)
}
