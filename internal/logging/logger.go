package logging

import (
	"fmt"
	"os"
)

type logMessage struct {
	message string
	json    bool
}

var logChan chan logMessage

func StartLogger() {
	logChan = make(chan logMessage, 1000)

	go func() {
		for msg := range logChan {
			if msg.json {
				fmt.Println(msg.message)
			} else {
				fmt.Fprintln(os.Stderr, msg.message)
			}
		}
	}()
}

func Printf(format string, args ...interface{}) {
	select {
	case logChan <- logMessage{message: fmt.Sprintf(format, args...), json: false}:
	default:
	}
}

func JSON(jsonStr string) {
	select {
	case logChan <- logMessage{message: jsonStr, json: true}:
	default:
	}
}