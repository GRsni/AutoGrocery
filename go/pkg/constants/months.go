package constants

import "time"

type SpanishMonth string

const (
	Enero      SpanishMonth = "Enero"
	Febrero    SpanishMonth = "Febrero"
	Marzo      SpanishMonth = "Marzo"
	Abril      SpanishMonth = "Abril"
	Mayo       SpanishMonth = "Mayo"
	Junio      SpanishMonth = "Junio"
	Julio      SpanishMonth = "Julio"
	Agosto     SpanishMonth = "Agosto"
	Septiembre SpanishMonth = "Septiembre"
	Octubre    SpanishMonth = "Octubre"
	Noviembre  SpanishMonth = "Noviembre"
	Diciembre  SpanishMonth = "Diciembre"
)

// spanishMonths is indexed by time.Month value (1 = January, 12 = December).
// Index 0 is intentionally empty so the slice aligns with time.Month values.
var spanishMonths = [...]SpanishMonth{
	"",
	Enero, Febrero, Marzo, Abril, Mayo, Junio,
	Julio, Agosto, Septiembre, Octubre, Noviembre, Diciembre,
}

// FromTimeMonth converts a time.Month into its Spanish name.
func FromTimeMonth(m time.Month) SpanishMonth {
	if m < time.January || m > time.December {
		return ""
	}
	return spanishMonths[m]
}
