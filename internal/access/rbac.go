package access

import (
	"fmt"
	"log/slog"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

type Permission struct {
	Resource string   `yaml:"resource"`
	Actions  []string `yaml:"actions"`
}

type Role struct {
	Description string       `yaml:"description"`
	Permissions []Permission `yaml:"permissions"`
}

type RBACPolicy struct {
	DefaultRole string              `yaml:"default_role"`
	Roles       map[string]Role     `yaml:"roles"`
	Users       map[string]struct { // Add user assignments
		Roles []string `yaml:"roles"`
	} `yaml:"users"`
	Inheritance map[string][]string `yaml:"inheritance"`
}

type RBAC struct {
	policy      *RBACPolicy
	userRoles   map[string][]string // userID -> roles
	mu          sync.RWMutex
	policyCache map[string]map[string]bool // userID -> "resource:action" -> allowed
}

var (
	rbacInstance *RBAC
	rbacOnce     sync.Once
)

// GetRBAC returns the singleton RBAC instance
func GetRBAC() *RBAC {
	rbacOnce.Do(func() {
		rbacInstance = &RBAC{
			userRoles:   make(map[string][]string),
			policyCache: make(map[string]map[string]bool),
		}
	})
	return rbacInstance
}

// LoadPolicy loads RBAC policy from YAML file
func (r *RBAC) LoadPolicy(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read policy file: %w", err)
	}

	var policy RBACPolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return fmt.Errorf("failed to parse policy file: %w", err)
	}

	r.mu.Lock()
	r.policy = &policy
	// Load user role assignments from policy
	r.userRoles = make(map[string][]string)
	for userID, userData := range policy.Users {
		r.userRoles[userID] = userData.Roles
	}
	r.policyCache = make(map[string]map[string]bool) // Clear cache
	r.mu.Unlock()

	slog.Info("RBAC policy loaded", "roles", len(policy.Roles), "users", len(policy.Users))
	return nil
}

// AssignRole assigns one or more roles to a user
func (r *RBAC) AssignRole(userID string, roles ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.userRoles[userID] = append(r.userRoles[userID], roles...)
	delete(r.policyCache, userID) // Invalidate cache for this user

	slog.Debug("Roles assigned", "userID", userID, "roles", roles)
}

// SetRoles replaces all roles for a user
func (r *RBAC) SetRoles(userID string, roles ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.userRoles[userID] = roles
	delete(r.policyCache, userID) // Invalidate cache for this user

	slog.Debug("Roles set", "userID", userID, "roles", roles)
}

// RemoveRole removes a role from a user
func (r *RBAC) RemoveRole(userID string, role string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	roles := r.userRoles[userID]
	newRoles := make([]string, 0, len(roles))
	for _, r := range roles {
		if r != role {
			newRoles = append(newRoles, r)
		}
	}
	r.userRoles[userID] = newRoles
	delete(r.policyCache, userID) // Invalidate cache

	slog.Debug("Role removed", "userID", userID, "role", role)
}

// GetUserRoles returns all roles for a user (including inherited)
func (r *RBAC) GetUserRoles(userID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if userID == "" {
		if r.policy != nil && r.policy.DefaultRole != "" {
			return []string{r.policy.DefaultRole}
		} else {
			slog.Warn("GetUserRoles: empty userID with no default role")
			return []string{}
		}
	}

	directRoles := r.userRoles[userID]

	// If user has no roles and default role is defined, use default role
	if len(directRoles) == 0 && r.policy != nil && r.policy.DefaultRole != "" {
		directRoles = []string{r.policy.DefaultRole}
	}

	allRoles := make(map[string]bool)

	for _, role := range directRoles {
		allRoles[role] = true
		// Add inherited roles
		r.addInheritedRoles(role, allRoles)
	}

	result := make([]string, 0, len(allRoles))
	for role := range allRoles {
		result = append(result, role)
	}
	return result
}

// addInheritedRoles recursively adds inherited roles
func (r *RBAC) addInheritedRoles(role string, roles map[string]bool) {
	if r.policy == nil || r.policy.Inheritance == nil {
		return
	}

	inherited := r.policy.Inheritance[role]
	for _, inheritedRole := range inherited {
		if !roles[inheritedRole] {
			roles[inheritedRole] = true
			r.addInheritedRoles(inheritedRole, roles)
		}
	}
}

// Can checks if a user can perform an action on a resource
func (r *RBAC) Can(userID, resource, action string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.policy == nil {
		slog.Warn("RBAC policy not loaded")
		return false
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", resource, action)
	if cache, exists := r.policyCache[userID]; exists {
		if allowed, found := cache[cacheKey]; found {
			return allowed
		}
	}

	// Get all roles for user
	roles := r.GetUserRoles(userID)
	allowed := false

	for _, roleName := range roles {
		role, exists := r.policy.Roles[roleName]
		if !exists {
			continue
		}

		for _, perm := range role.Permissions {
			// Check wildcard resource
			if perm.Resource == "*" || perm.Resource == resource {
				// Check wildcard action or specific action
				for _, act := range perm.Actions {
					if act == "*" || act == action {
						allowed = true
						break
					}
				}
			}
			if allowed {
				break
			}
		}
		if allowed {
			break
		}
	}

	// Cache the result
	if r.policyCache[userID] == nil {
		r.policyCache[userID] = make(map[string]bool)
	}
	r.policyCache[userID][cacheKey] = allowed

	return allowed
}

// Require is a helper that panics if user doesn't have permission
func (r *RBAC) Require(userID, resource, action string) {
	if !r.Can(userID, resource, action) {
		panic(fmt.Sprintf("user %s lacks permission %s:%s", userID, resource, action))
	}
}
