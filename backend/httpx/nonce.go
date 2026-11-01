package httpx

import "context"

type nonceKey struct{}

// WithNonce stores the per-request CSP nonce in the context.
func WithNonce(ctx context.Context, nonce string) context.Context {
	return context.WithValue(ctx, nonceKey{}, nonce)
}

// NonceFrom returns the CSP nonce of the request ("" if none).
func NonceFrom(ctx context.Context) string {
	n, _ := ctx.Value(nonceKey{}).(string)
	return n
}
