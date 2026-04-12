package auth

// Permission constants
const (
	// User management permissions
	PermUsersCreate = "users:create"
	PermUsersRead   = "users:read"
	PermUsersUpdate = "users:update"
	PermUsersDelete = "users:delete"

	// Rule management permissions
	PermRulesCreate = "rules:create"
	PermRulesRead   = "rules:read"
	PermRulesUpdate = "rules:update"
	PermRulesDelete = "rules:delete"

	// Alert permissions
	PermAlertsRead       = "alerts:read"
	PermAlertsAcknowledge = "alerts:acknowledge"

	// Configuration permissions
	PermConfigRead   = "config:read"
	PermConfigUpdate = "config:update"

	// Audit log permissions
	PermAuditRead = "audit:read"
)

// Role constants
const (
	RoleAdmin    = "admin"
	RoleAnalyst  = "analyst"
	RoleViewer   = "viewer"
)

// PermissionMatrix defines which roles have which permissions
var PermissionMatrix = map[string][]string{
	RoleAdmin: {
		PermUsersCreate,
		PermUsersRead,
		PermUsersUpdate,
		PermUsersDelete,
		PermRulesCreate,
		PermRulesRead,
		PermRulesUpdate,
		PermRulesDelete,
		PermAlertsRead,
		PermAlertsAcknowledge,
		PermConfigRead,
		PermConfigUpdate,
		PermAuditRead,
	},
	RoleAnalyst: {
		PermUsersRead,
		PermRulesRead,
		PermAlertsRead,
		PermAlertsAcknowledge,
		PermConfigRead,
		PermAuditRead,
	},
	RoleViewer: {
		PermUsersRead,
		PermRulesRead,
		PermAlertsRead,
		PermConfigRead,
	},
}

// PermissionChecker provides permission checking logic
type PermissionChecker struct {
	matrix map[string][]string
}

// NewPermissionChecker creates a new permission checker
func NewPermissionChecker() *PermissionChecker {
	return &PermissionChecker{
		matrix: PermissionMatrix,
	}
}

// HasPermission checks if a role has a specific permission
func (pc *PermissionChecker) HasPermission(role, permission string) bool {
	permissions, exists := pc.matrix[role]
	if !exists {
		return false
	}

	for _, p := range permissions {
		if p == permission {
			return true
		}
	}

	return false
}

// GetPermissionsForRole returns all permissions for a role
func (pc *PermissionChecker) GetPermissionsForRole(role string) []string {
	if permissions, exists := pc.matrix[role]; exists {
		return append([]string{}, permissions...) // Return copy
	}
	return []string{}
}

// PermissionDescriptions provides human-readable descriptions of permissions
var PermissionDescriptions = map[string]string{
	PermUsersCreate: "Create new users",
	PermUsersRead:   "View user information",
	PermUsersUpdate: "Update user roles and settings",
	PermUsersDelete: "Delete or suspend users",

	PermRulesCreate: "Create detection rules",
	PermRulesRead:   "View detection rules",
	PermRulesUpdate: "Update detection rules",
	PermRulesDelete: "Delete detection rules",

	PermAlertsRead:        "View alerts and detections",
	PermAlertsAcknowledge: "Acknowledge and suppress alerts",

	PermConfigRead:   "View system configuration",
	PermConfigUpdate: "Modify system configuration",

	PermAuditRead: "View audit logs",
}

// GetPermissionDescription returns a human-readable description for a permission
func GetPermissionDescription(permission string) string {
	if desc, exists := PermissionDescriptions[permission]; exists {
		return desc
	}
	return "Unknown permission"
}

// RoleHierarchy defines role inheritance (higher level includes lower level capabilities)
// In some systems, admin > analyst > viewer
// For Argus, roles are orthogonal (admin != analyst + viewer)
// But we can still define which roles can manage which roles
var RoleManagementMatrix = map[string][]string{
	RoleAdmin: {RoleAdmin, RoleAnalyst, RoleViewer},
	RoleAnalyst: {},
	RoleViewer: {},
}

// CanManageRole checks if one role can manage another role
func CanManageRole(managerRole, targetRole string) bool {
	manageable, exists := RoleManagementMatrix[managerRole]
	if !exists {
		return false
	}

	for _, role := range manageable {
		if role == targetRole {
			return true
		}
	}

	return false
}
