package round

import "testing"

func TestKindString(t *testing.T) {
	cases := []struct {
		name string
		kind Kind
		want string
	}{
		{"undefined", KindUndefined, "undefined"},
		{"questions", KindQuestions, "questions"},
		{"legend", KindLegend, "legend"},
		{"unknown", Kind(99), "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.kind.String(); got != tc.want {
				t.Fatalf("Kind(%d).String() = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func TestSchemaVersionIsOne(t *testing.T) {
	if SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", SchemaVersion)
	}
}
