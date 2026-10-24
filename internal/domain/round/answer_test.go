package round

import "testing"

func TestAnswerString(t *testing.T) {
	choices := []Choice{{Text: "Postgres"}, {Text: "SQLite"}}
	cases := []struct {
		name string
		ans  Answer
		want string
	}{
		{
			name: "skipped",
			ans:  Answer{ChoiceIndex: ChoiceIndexNone},
			want: "",
		},
		{
			name: "other with comments",
			ans:  Answer{ChoiceIndex: ChoiceIndexOther, Comments: []Markdown{"I want **BoltDB**", "actually twice"}},
			want: "I want **BoltDB**\n\nactually twice",
		},
		{
			name: "listed choice",
			ans:  Answer{ChoiceIndex: 2},
			want: "SQLite",
		},
		{
			name: "listed choice with comments",
			ans:  Answer{ChoiceIndex: 1, Comments: []Markdown{"vendored", "and WAL only"}},
			want: "Postgres\n\nvendored\n\nand WAL only",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ans.String(choices); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
