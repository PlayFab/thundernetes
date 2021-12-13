package log

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

func Debug(msg string, args ...interface{}) {
	log.WithFields(getLogFields(args...)).Debug(msg)
}

func Info(msg string, args ...interface{}) {
	log.WithFields(getLogFields(args...)).Info(msg)
}

func Warn(msg string, args ...interface{}) {
	log.WithFields(getLogFields(args...)).Warn(msg)
}

func Error(msg string, args ...interface{}) {
	log.WithFields(getLogFields(args...)).Error(msg)
}

func Fatal(msg string, args ...interface{}) {
	log.WithFields(getLogFields(args...)).Fatal(msg)
}

func getLogFields(args ...interface{}) log.Fields {
	fields := make(map[string]interface{})
	for i := 1; i < len(args); i += 2 {
		fields[fmt.Sprintf("%v", args[i-1])] = args[i]
	}

	return fields
}
