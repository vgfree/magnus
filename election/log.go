package election

import (
	"io/ioutil"
	"log"
	"os"
)

var (
	logger *log.Logger
	debug  *log.Logger
)

// SetLogger sets the package logger. If `l` is nil then a default logger will
// be created that outputs to stdout. If `dbg` is true then debug log output
// will be enabled.
func SetLogger(l *log.Logger, dbg bool) {
	if l == nil {
		l = log.New(os.Stdout, "", log.LstdFlags)
	}
	logger = l
	if dbg {
		debug = l
	} else {
		debug = log.New(ioutil.Discard, "", log.LstdFlags)
	}
}

// Initialize the logger for stdout. Disable debugging.
func init() {
	SetLogger(log.New(os.Stdout, "", log.LstdFlags), false)
}
