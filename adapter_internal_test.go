package sqlview

import "testing"

func TestBuildSmartOrderBy(t *testing.T) {
	tests := []struct {
		name    string
		columns []string
		autoInc []string
		want    string
	}{
		{
			name:    "empty columns",
			columns: nil,
			want:    "",
		},
		{
			name:    "no matching columns",
			columns: []string{"foo", "bar"},
			want:    "",
		},
		{
			name:    "id only",
			columns: []string{"id", "name"},
			want:    "id DESC",
		},
		{
			name:    "updated_at beats id",
			columns: []string{"id", "updated_at"},
			want:    "updated_at DESC",
		},
		{
			name:    "updated_at beats created_at",
			columns: []string{"created_at", "updated_at"},
			want:    "updated_at DESC",
		},
		{
			name:    "case-insensitive match preserves original casing",
			columns: []string{"UpdatedAt", "Id"},
			want:    "Id DESC",
		},
		{
			name:    "auto-increment beats id when they differ",
			columns: []string{"id", "seq_no"},
			autoInc: []string{"seq_no"},
			want:    "seq_no DESC",
		},
		{
			name:    "auto-increment id produces id (consistent with legacy behavior)",
			columns: []string{"id", "name"},
			autoInc: []string{"id"},
			want:    "id DESC",
		},
		{
			name:    "auto-increment used when id column absent",
			columns: []string{"uid", "name"},
			autoInc: []string{"uid"},
			want:    "uid DESC",
		},
		{
			name:    "time column still outranks auto-increment",
			columns: []string{"seq_no", "updated_at"},
			autoInc: []string{"seq_no"},
			want:    "updated_at DESC",
		},
		{
			name:    "auto-increment ignored if empty string",
			columns: []string{"id"},
			autoInc: []string{""},
			want:    "id DESC",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildSmartOrderBy(tc.columns, tc.autoInc)
			if got != tc.want {
				t.Errorf("buildSmartOrderBy(%v, %v) = %q, want %q", tc.columns, tc.autoInc, got, tc.want)
			}
		})
	}
}
