package user

// Role represents the authorization level of a user within the system.
type Role string

const (
	RoleAdmin            Role = "admin"
	RoleLandlord         Role = "landlord"
	RoleTenant           Role = "tenant"
	RoleMaintenanceStaff Role = "maintenance_staff"
)

// IsValid reports whether r is one of the defined role constants.
func (r Role) IsValid() bool {
	switch r {
	case RoleAdmin, RoleLandlord, RoleTenant, RoleMaintenanceStaff:
		return true
	}
	return false
}

// String implements fmt.Stringer.
func (r Role) String() string {
	return string(r)
}
