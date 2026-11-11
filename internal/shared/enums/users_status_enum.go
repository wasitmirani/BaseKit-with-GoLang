package enums

type UserStatus string

const (
    UserStatusActive   UserStatus = "active"
    UserStatusInactive UserStatus = "inactive"
    UserStatusBlocked  UserStatus = "blocked"
    UserStatusPending  UserStatus = "pending"
)