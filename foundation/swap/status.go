package swap

const (
	Inactive = iota
	Initiated
	Audited
	AuditPending
	AuditFailed
	Redeemed
	AuditedSecret
	Refunded
	RefundFailed
	Cancelled
	Expired
)

// StatusUpdate shows the status change of a swap.
type StatusUpdate struct {
	ID   SwapID `json:"id"`
	Code int    `json:"code"`
}

// NewStatusUpdate creates a new `StatusUpdate` with given swap ID and status.
func NewStatusUpdate(id SwapID, status int) StatusUpdate {
	return StatusUpdate{id, status}
}
