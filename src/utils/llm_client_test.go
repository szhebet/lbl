package utils

import (
	"strings"
	"testing"
)

func TestNormalizeBookListText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "unify quotes",
			in:   "«Война и мир» — «Толстой»",
			want: `"Война и мир" - "Толстой"`,
		},
		{
			name: "strip emphasis markup",
			in:   "**Лев Толстой** — _Война и мир_",
			want: "Лев Толстой - Война и мир",
		},
		{
			name: "strip special bullets",
			in:   "• Пушкин — •Дубровский",
			want: "Пушкин - Дубровский",
		},
		{
			name: "collapse whitespace",
			in:   "Лев   Толстой —  Война  и мир  ",
			want: "Лев Толстой - Война и мир",
		},
		{
			name: "preserve line breaks",
			in:   "Лев Толстой — Война и мир\nА.С. Пушкин — Дубровский\n\nМ.Ю. Лермонтов — Герой нашего времени",
			want: "Лев Толстой - Война и мир\nА.С. Пушкин - Дубровский\nМ.Ю. Лермонтов - Герой нашего времени",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeBookListText(tc.in)
			if got != tc.want {
				t.Errorf("NormalizeBookListText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeBookListTextKeepsPlainTitles(t *testing.T) {
	in := "Чехов — «Вишнёвый сад»"
	got := NormalizeBookListText(in)
	if strings.Contains(got, "«") || strings.Contains(got, "»") {
		t.Errorf("expected quotes unified, got %q", got)
	}
	if !strings.Contains(got, `"Вишнёвый сад"`) {
		t.Errorf("expected ASCII quotes around title, got %q", got)
	}
}
