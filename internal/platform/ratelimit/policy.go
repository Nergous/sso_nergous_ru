package ratelimit

import "time"

type PolicyName string

func (p PolicyName) String() string {
	return string(p)
}

const (
	LoginPerIP            PolicyName = "login_per_ip"
	LoginPerUsername      PolicyName = "login_per_username"
	ResetPerIP            PolicyName = "reset_per_ip"
	ResetPerEmail         PolicyName = "reset_per_email"
	ServiceAuthPerClient  PolicyName = "service_auth_per_client"
	ChangePasswordPerUser PolicyName = "change_password_per_user"
)

type Policy struct {
	Name      PolicyName
	RPS       float64
	Burst     int
	IdleEvict time.Duration
}

func NewPolicy(name PolicyName, rps float64, burst int, idleEvict time.Duration) *Policy {
	return &Policy{Name: name, RPS: rps, Burst: burst, IdleEvict: idleEvict}
}
