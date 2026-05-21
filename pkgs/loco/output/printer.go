package output

import "fmt"

type Printer interface {
	Printf(format string, a ...any) (n int, err error)
}

type ConsolePrinter struct{}

func (c ConsolePrinter) Printf(format string, a ...any) (n int, err error) {
	return fmt.Printf(format, a...)
}
