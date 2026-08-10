package postgres

// NullIfEmpty maps empty strings to SQL NULL for optional varchar columns.
// Non-empty values are returned as *string so pgx encodes them as text.
func NullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
