package extract

type logger struct {
	verbose bool
	Printer
}

func (l logger) debugf(format string, i ...interface{}) {
	if l.verbose {
		l.PrintErrf("[hack] "+format+"\n", i...)
	}
}
