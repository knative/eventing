package retcode

import "hash/crc32"

var (
	// LowerBound is the lower bound of the POSIX retcode range. Use this to
	// configure the package.
	LowerBound = 1
	// UpperBound is the upper bound of the POSIX retcode range. Use this to
	// configure the package.
	UpperBound = 255
)

// Calc will calculate an POSIX retcode from an error.
func Calc(err error) int {
	if err == nil {
		return 0
	}
	if r, ok := err.(retcodeErr); ok {
		return r.Retcode()
	}
	upper := UpperBound - LowerBound
	return int(crc32.ChecksumIEEE([]byte(err.Error())))%upper + LowerBound
}

type retcodeErr interface {
	Retcode() int
}
