package models

import (
	"testing"
	"time"
)

func TestTimeAgo(t *testing.T) {
	now := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		from time.Time
		want string
	}{
		{"ahora mismo", now, "justo ahora"},
		{"hace 30 segundos", now.Add(-30 * time.Second), "justo ahora"},
		{"en el futuro por desincronización", now.Add(30 * time.Second), "justo ahora"},
		{"hace 1 minuto", now.Add(-1 * time.Minute), "hace 1 minuto"},
		{"hace 2 minutos", now.Add(-2 * time.Minute), "hace 2 minutos"},
		{"hace 59 minutos", now.Add(-59 * time.Minute), "hace 59 minutos"},
		{"hoy, hace 2 horas", now.Add(-2 * time.Hour), "hoy"},
		{"hoy, justo tras medianoche", time.Date(2024, 6, 15, 0, 1, 0, 0, time.UTC), "hoy"},
		{"ayer", now.AddDate(0, 0, -1), "ayer"},
		{"hace 2 días", now.AddDate(0, 0, -2), "hace 2 días"},
		{"hace 6 días", now.AddDate(0, 0, -6), "hace 6 días"},
		{"hace 1 semana", now.AddDate(0, 0, -7), "hace 1 semana"},
		{"hace 3 semanas", now.AddDate(0, 0, -21), "hace 3 semanas"},
		{"hace 1 mes", now.AddDate(0, 0, -30), "hace 1 mes"},
		{"hace 2 meses", now.AddDate(0, 0, -60), "hace 2 meses"},
		{"hace 1 año", now.AddDate(0, 0, -365), "hace 1 año"},
		{"hace 2 años", now.AddDate(0, 0, -730), "hace 2 años"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := TimeAgo(tc.from, now)
			if got != tc.want {
				t.Errorf("TimeAgo(%v, %v) = %q, want %q", tc.from, now, got, tc.want)
			}
		})
	}
}
