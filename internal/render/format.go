package render

import (
	"math"
	"strconv"
	"strings"

	"github.com/nebulaeworks/doxythings/internal/model"
)

/**
 * FormatTime formats a duration in nanoseconds in the largest unit it
 * reaches, e.g. "1.657 µs" or "7.2 ns"; "-" for NaN.
 *
 * @param ns The duration, in nanoseconds.
 * @return The formatted duration.
 */
func FormatTime(ns float64) string {
	if math.IsNaN(ns) {
		return "-"
	}
	for _, u := range []struct {
		scale float64
		unit  string
	}{{1e9, "s"}, {1e6, "ms"}, {1e3, "µs"}} {
		if ns >= u.scale {
			return FormatNumber(ns/u.scale, 3) + " " + u.unit
		}
	}
	return FormatNumber(ns, 1) + " ns"
}

/**
 * FormatNumber formats v with the given number of decimals and comma
 * thousands separators, e.g. "12,345.68"; "nan" or "inf" when not finite.
 *
 * @param v The number.
 * @param decimals How many decimals to show.
 * @return The formatted number.
 */
func FormatNumber(v float64, decimals int) string {
	switch {
	case math.IsNaN(v):
		return "nan"
	case math.IsInf(v, 1):
		return "inf"
	case math.IsInf(v, -1):
		return "-inf"
	}
	text := strconv.FormatFloat(v, 'f', decimals, 64)
	whole, fraction, hasFraction := strings.Cut(text, ".")
	if hasFraction {
		return groupThousands(whole) + "." + fraction
	}
	return groupThousands(whole)
}

/**
 * FormatCount formats an integer with comma thousands separators, e.g.
 * "82,348,860".
 *
 * @param n The integer.
 * @return The formatted integer.
 */
func FormatCount(n int64) string {
	return groupThousands(strconv.FormatInt(n, 10))
}

/**
 * groupThousands inserts commas between every three digits of an integer's
 * decimal text, keeping any leading sign.
 *
 * @param digits An integer's decimal digits, optionally signed.
 * @return The digits with commas.
 */
func groupThousands(digits string) string {
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}
	return sign + b.String()
}

/**
 * timeColumns are the headers of the cells timeCells produces.
 */
var timeColumns = []string{"CPU mean", "CPU median", "CPU stddev", "CPU min", "CPU max", "Real mean"}

/**
 * timeCells formats a run's times, matching timeColumns.
 *
 * @param run The run.
 * @return One cell per time column.
 */
func timeCells(run *model.Run) []string {
	cpu := run.CPU()
	return []string{
		FormatTime(cpu.Mean), FormatTime(cpu.Median), FormatTime(cpu.StdDev),
		FormatTime(cpu.Min), FormatTime(cpu.Max), FormatTime(run.Real().Mean),
	}
}
