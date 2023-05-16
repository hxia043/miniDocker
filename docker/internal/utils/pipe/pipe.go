package pipe

import "os"

func NewPipe() (*os.File, *os.File, error) {
	readPipe, writePipe, err := os.Pipe()
	return readPipe, writePipe, err
}
