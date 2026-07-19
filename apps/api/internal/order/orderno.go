package order

import (
	"crypto/rand"
	"fmt"
	"time"
)

// generateOrderNo returns a human-friendly, sortable order number:
// ORD-YYYYMMDD-XXXXXX where the suffix is a random base36-ish token.
// Uniqueness is ultimately enforced by the orders.order_no UNIQUE constraint.
func generateOrderNo() string {
	const alphabet = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ" // no I/O to avoid confusion
	suffix := make([]byte, 6)
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		// Fallback to time-derived bytes; still unique enough with the DB constraint.
		now := time.Now().UnixNano()
		for i := range buf {
			buf[i] = byte(now >> (uint(i) * 8))
		}
	}
	for i, b := range buf {
		suffix[i] = alphabet[int(b)%len(alphabet)]
	}
	return fmt.Sprintf("ORD-%s-%s", time.Now().Format("20060102"), string(suffix))
}
