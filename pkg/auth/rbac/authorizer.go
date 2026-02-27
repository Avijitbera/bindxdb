package rbac

import (
	"bindxdb/pkg/auth"
	"context"
	"strings"
	"sync"
)

type RBACAuthorizer struct {
	roles       map[string]*auth.Role
	userRoles   map[string][]string
	permissions map[string]map[string]string
	mu          sync.RWMutex
}

func NewRBACAuthorizer() *RBACAuthorizer {
	return &RBACAuthorizer{
		roles:       make(map[string]*auth.Role),
		userRoles:   make(map[string][]string),
		permissions: make(map[string]map[string]string),
	}
}

func (r *RBACAuthorizer) AddRole(role *auth.Role) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.roles[role.Name] = role

	for _, perm := range role.Permissions {
		if _, ok := r.permissions[perm]; !ok {
			r.permissions[perm] = make(map[string]string)
		}
		r.permissions[perm]["*"] = "allow"
	}
}

func (r *RBACAuthorizer) AssignRole(userID string, roleName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.userRoles[userID] = append(r.userRoles[userID], roleName)
}

func (r *RBACAuthorizer) Authorize(ctx context.Context, authCtx *auth.AuthContext, resource string, action string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, roleName := range authCtx.Roles {
		role, exists := r.roles[roleName]
		if !exists {
			continue
		}
		for _, perm := range role.Permissions {
			if r.matchesPermission(perm, resource, action) {
				return true, nil
			}
		}
	}

	for _, perm := range authCtx.Permissions {
		if r.matchesPermission(perm.Resource+":"+perm.Action, resource, action) {
			return perm.Effect == "allow", nil
		}
	}

	return false, nil
}

func (r *RBACAuthorizer) GetPermissions(ctx context.Context, authCtx *auth.AuthContext) ([]auth.Permission, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	permissions := make([]auth.Permission, 0)

	seen := make(map[string]bool)

	for _, roleName := range authCtx.Roles {
		role, exists := r.roles[roleName]
		if !exists {
			continue
		}

		for _, perm := range role.Permissions {
			if !seen[perm] {
				resource, action := r.parsePermission(perm)
				permissions = append(permissions, auth.Permission{
					Resource: resource,
					Action:   action,
					Effect:   "allow",
				})
				seen[perm] = true
			}
		}
	}

	for _, perm := range authCtx.Permissions {
		key := perm.Resource + ":" + perm.Action
		if !seen[key] {
			permissions = append(permissions, perm)
			seen[key] = true
		}
	}

	return permissions, nil

}

func (r *RBACAuthorizer) matchesPermission(perm, resource, action string) bool {
	parts := strings.Split(perm, ":")
	if len(parts) != 2 {
		return false
	}

	permResource := parts[0]
	permAction := parts[1]

	resourceMatch := permResource == "*" || permResource == resource
	if !resourceMatch {
		return false
	}
	actionMatch := permAction == "*" || permAction == action
	return actionMatch

}

func (r *RBACAuthorizer) parsePermission(perm string) (string, string) {
	parts := strings.Split(perm, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return perm, "*"
}
