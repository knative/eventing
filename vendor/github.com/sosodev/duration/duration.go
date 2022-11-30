package duration

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Duration holds all the smaller units that make up the duration
type Duration struct {
	Years   float64
	Months  float64
	Weeks   float64
	Days    float64
	Hours   float64
	Minutes float64
	Seconds float64
}

const (
	parsingPeriod = iota
	parsingTime
)

var (
	// ErrUnexpectedInput is returned when an input in the duration string does not match expectations
	ErrUnexpectedInput = errors.New("unexpected input")
)

// Parse attempts to parse the given duration string into a *Duration
// if parsing fails an error is returned instead
func Parse(d string) (*Duration, error) {
	state := parsingPeriod
	duration := &Duration{}
	num := ""
	var err error

	for _, char := range d {
		switch char {
		case 'P':
			state = parsingPeriod
		case 'T':
			state = parsingTime
		case 'Y':
			if state != parsingPeriod {
				return nil, ErrUnexpectedInput
			}

			duration.Years, err = strconv.ParseFloat(num, 64)
			if err != nil {
				return nil, err
			}
			num = ""
		case 'M':
			if state == parsingPeriod {
				duration.Months, err = strconv.ParseFloat(num, 64)
				if err != nil {
					return nil, err
				}
				num = ""
			} else if state == parsingTime {
				duration.Minutes, err = strconv.ParseFloat(num, 64)
				if err != nil {
					return nil, err
				}
				num = ""
			}
		case 'W':
			if state != parsingPeriod {
				return nil, ErrUnexpectedInput
			}

			duration.Weeks, err = strconv.ParseFloat(num, 64)
			if err != nil {
				return nil, err
			}
			num = ""
		case 'D':
			if state != parsingPeriod {
				return nil, ErrUnexpectedInput
			}

			duration.Days, err = strconv.ParseFloat(num, 64)
			if err != nil {
				return nil, err
			}
			num = ""
		case 'H':
			if state != parsingTime {
				return nil, ErrUnexpectedInput
			}

			duration.Hours, err = strconv.ParseFloat(num, 64)
			if err != nil {
				return nil, err
			}
			num = ""
		case 'S':
			if state != parsingTime {
				return nil, ErrUnexpectedInput
			}

			duration.Seconds, err = strconv.ParseFloat(num, 64)
			if err != nil {
				return nil, err
			}
			num = ""
		default:
			if unicode.IsNumber(char) || char == '.' {
				num += string(char)
				continue
			}

			return nil, ErrUnexpectedInput
		}
	}

	return duration, nil
}

// ToTimeDuration converts the *Duration to the standard library's time.Duration
// note that for *Duration's with period values of a month or year that the duration becomes a bit fuzzy
// since obviously those things vary month to month and year to year
// I used the values that Google's search provided me with as I couldn't find anything concrete on what they should be
func (duration *Duration) ToTimeDuration() time.Duration {
	var timeDuration time.Duration

	timeDuration += time.Duration(math.Round(duration.Years * 3.154e+16))
	timeDuration += time.Duration(math.Round(duration.Months * 2.628e+15))
	timeDuration += time.Duration(math.Round(duration.Weeks * 6.048e+14))
	timeDuration += time.Duration(math.Round(duration.Days * 8.64e+13))
	timeDuration += time.Duration(math.Round(duration.Hours * 3.6e+12))
	timeDuration += time.Duration(math.Round(duration.Minutes * 6e+10))
	timeDuration += time.Duration(math.Round(duration.Seconds * 1e+9))

	return timeDuration
}

// String returns the ISO8601 duration string for the *Duration
func (duration *Duration) String() string {
	d := ""

	appendD := func(designator string, value float64, isTime bool) {
		if !strings.Contains(d, "P") && !isTime {
			d += "P"
		}

		if !strings.Contains(d, "T") && isTime {
			d += "T"
		}

		d += fmt.Sprintf("%s%s", strconv.FormatFloat(value, 'f', -1, 64), designator)
	}

	if duration.Years != 0 {
		appendD("Y", duration.Years, false)
	}

	if duration.Months != 0 {
		appendD("M", duration.Months, false)
	}

	if duration.Weeks != 0 {
		appendD("W", duration.Weeks, false)
	}

	if duration.Days != 0 {
		appendD("D", duration.Days, false)
	}

	if duration.Hours != 0 {
		appendD("H", duration.Hours, true)
	}

	if duration.Minutes != 0 {
		appendD("M", duration.Minutes, true)
	}

	if duration.Seconds != 0 {
		appendD("S", duration.Seconds, true)
	}

	return d
}
