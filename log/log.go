package log

import (
	"io/ioutil"
	"os"

	logging "github.com/op/go-logging"
)

var (
	logger      = logging.MustGetLogger("xc")
	logfile     *os.File
	initialized = false

	// Debug proxy
	Debug = logger.Debug
	// Debugf proxy
	Debugf = logger.Debugf
)

// Initialize logger
func Initialize(logFilename string) error {
	if logFilename == "" {
		setupNullLogger()
		return nil
	}

	logfile, err := os.OpenFile(logFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		setupNullLogger()
		return err
	}
	backend := logging.NewLogBackend(logfile, "", 0)
	format := logging.MustStringFormatter(
		`[%{time:15:04:05.000}] %{message}`,
	)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)
	logger.Debug("logger initialized")
	initialized = true
	return nil
}

func setupNullLogger() {
	backend := logging.NewLogBackend(ioutil.Discard, "", 0)
	logging.SetBackend(backend)
}
