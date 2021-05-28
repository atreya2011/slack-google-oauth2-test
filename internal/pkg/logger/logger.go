package logger

import (
	"fmt"
	"log"
	"os"
)

func New(fileName string) (logger *log.Logger, err error) {
	// init log file
	logFile, err := os.OpenFile(fileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		err = fmt.Errorf("unable to create log file: %s\n", err)
		return
	}
	logger = log.New(logFile, "", log.LstdFlags)
	return
}
