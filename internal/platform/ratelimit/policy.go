package ratelimit

import "time"

// PolicyName is the stable identifier of a rate-limit policy. Its string value
// is also the configuration key, so these constants must stay in sync with the
// policy keys used in config and in the interceptor binding table.
type PolicyName string

// String returns the policy name as a plain string, implementing fmt.Stringer.
func (p PolicyName) String() string {
	return string(p)
}

// Known policy names. Each guards one dimension of a defended RPC — for
// example login is limited both per source IP and per username.
const (
	LoginPerIP            PolicyName = "login_per_ip"
	LoginPerUsername      PolicyName = "login_per_username"
	ResetPerIP            PolicyName = "reset_per_ip"
	ResetPerEmail         PolicyName = "reset_per_email"
	ServiceAuthPerClient  PolicyName = "service_auth_per_client"
	ChangePasswordPerUser PolicyName = "change_password_per_user"
)

// Policy is the token-bucket configuration a KeyedLimiter applies to every key
// it tracks.
type Policy struct {
	// Name identifies the policy and is used as the limiter map key.
	Name PolicyName

	// RPS is the sustained refill rate, in requests per second.
	RPS float64

	// Burst is the bucket size: how many requests may arrive at once before
	// the sustained rate begins to apply.
	Burst int

	// IdleEvict is how long a per-key bucket may go unused before the
	// background sweep removes it to reclaim memory.
	IdleEvict time.Duration
}
