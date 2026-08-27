package http

import "testing"

// TestSearchContentHasMore covers the pure pagination-hint derivation
// landed in Batch 3E. The helper is intentionally insensitive to the
// post-enforcement `len(filtered)` count — see the contract comment in
// the SearchContent handler for the rationale (Option 1: pre-
// enforcement-only).
func TestSearchContentHasMore(t *testing.T) {
	tests := []struct {
		name   string
		offset int
		limit  int
		total  int
		want   bool
	}{
		// --- baseline pagination cases (mode-agnostic) -------------------
		{
			name:   "first page of multi-page result set",
			offset: 0, limit: 20, total: 138,
			want: true, // 0 + 20 < 138
		},
		{
			name:   "middle page still has more",
			offset: 20, limit: 20, total: 138,
			want: true,
		},
		{
			name:   "last full page exhausts total",
			offset: 120, limit: 20, total: 138,
			want: false, // 120 + 20 = 140 >= 138
		},
		{
			name:   "exact-fit final page",
			offset: 100, limit: 20, total: 120,
			want: false, // 100 + 20 == 120, no further offset
		},
		{
			name:   "single-page result set smaller than limit",
			offset: 0, limit: 20, total: 5,
			want: false,
		},

		// --- enforce-mode shrunk-page cases ------------------------------
		// Per Batch 3C runtime evidence the response can carry total=4
		// while contents=[] when every projected row was evaluator-denied.
		// The helper is only given (offset, limit, total) — it intentionally
		// does NOT consult len(filtered), so its truth value is identical
		// across shadow and enforce modes for the same SQL candidate set.
		{
			name:   "enforce shrunk full page still reports has_more inside total",
			offset: 0, limit: 20, total: 138,
			want: true,
		},
		{
			name:   "enforce fully-dropped middle page still has_more (later pages may have visible rows)",
			offset: 20, limit: 20, total: 138,
			want: true,
		},
		{
			name:   "enforce shrunk last page no more",
			offset: 120, limit: 20, total: 138,
			want: false,
		},

		// --- safety guards -----------------------------------------------
		{
			name:   "limit zero (no pagination requested) coerces to false",
			offset: 0, limit: 0, total: 999,
			want: false,
		},
		{
			name:   "negative limit coerces to false",
			offset: 0, limit: -1, total: 999,
			want: false,
		},
		{
			name:   "negative offset normalised to zero",
			offset: -5, limit: 20, total: 30,
			want: true, // 0 + 20 < 30
		},
		{
			name:   "zero total never reports has_more",
			offset: 0, limit: 20, total: 0,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := searchContentHasMore(tt.offset, tt.limit, tt.total)
			if got != tt.want {
				t.Errorf(
					"searchContentHasMore(offset=%d, limit=%d, total=%d) = %v; want %v",
					tt.offset, tt.limit, tt.total, got, tt.want,
				)
			}
		})
	}
}


