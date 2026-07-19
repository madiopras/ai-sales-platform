package payment

import (
	"crypto/rand"
	"fmt"
	"time"
)

// generateInvoiceNo returns a human-friendly, sortable invoice number:
// INV-YYYYMMDD-XXXXXX where the suffix is a random base32-ish token.
// Uniqueness is ultimately enforced by the invoices.invoice_no UNIQUE constraint;
// the caller retries on ErrInvoiceAlreadyExists conflicts if the (rare)
// collision occurs.
func generateInvoiceNo() string {
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
	return fmt.Sprintf("INV-%s-%s", time.Now().Format("20060102"), string(suffix))
}
