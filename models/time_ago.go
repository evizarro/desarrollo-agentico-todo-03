package models

import (
	"fmt"
	"time"
)

// TimeAgo describe, en español y en lenguaje natural, cuánto tiempo ha
// pasado entre from y to (to es normalmente time.Now()).
func TimeAgo(from, to time.Time) string {
	elapsed := to.Sub(from)
	if elapsed < time.Minute {
		return "justo ahora"
	}
	if elapsed < time.Hour {
		minutes := int(elapsed.Minutes())
		return fmt.Sprintf("hace %d %s", minutes, pluralize(minutes, "minuto", "minutos"))
	}

	days := daysBetween(from, to)
	switch {
	case days == 0:
		return "hoy"
	case days == 1:
		return "ayer"
	case days < 7:
		return fmt.Sprintf("hace %d %s", days, pluralize(days, "día", "días"))
	case days < 30:
		weeks := days / 7
		return fmt.Sprintf("hace %d %s", weeks, pluralize(weeks, "semana", "semanas"))
	case days < 365:
		months := days / 30
		return fmt.Sprintf("hace %d %s", months, pluralize(months, "mes", "meses"))
	default:
		years := days / 365
		return fmt.Sprintf("hace %d %s", years, pluralize(years, "año", "años"))
	}
}

// daysBetween calcula la diferencia de días de calendario (en UTC) entre
// from y to, independientemente de la hora del día.
func daysBetween(from, to time.Time) int {
	from = from.UTC()
	to = to.UTC()
	fromDate := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	toDate := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return int(toDate.Sub(fromDate).Hours() / 24)
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
