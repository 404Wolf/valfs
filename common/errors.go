package common

import (
	"fmt"
	"log"
	"net/http"
)

func ReportErrorResp(
	format string,
	resp *http.Response,
	args ...interface{},
) string {
	allArgs := make([]interface{}, len(args)+1)
	copy(allArgs, args)
	allArgs[len(allArgs)-1] = resp
	message := fmt.Sprintf(format+", Response: %v", allArgs...)
	log.Print(message)
	return message
}

func ReportError(format string, err error, args ...interface{}) string {
	allArgs := make([]interface{}, len(args)+1)
	copy(allArgs, args)
	allArgs[len(allArgs)-1] = err
	message := fmt.Sprintf(format+": %v", allArgs...)
	log.Print(message)
	return message
}
