package gates

import "strconv"

func fmtInt(i int) string { return strconv.Itoa(i) }

func fmtPct(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) + "%" }
