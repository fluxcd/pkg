package masktoken

import (
	"fmt"
	"regexp"
)

func MaskTokenFromString(log string, token string) string {
	if token == "" {
		return log
	}

	re, compileErr := regexp.Compile(fmt.Sprintf("%s*", regexp.QuoteMeta(token)))
	if compileErr != nil {
		newErrStr := fmt.Sprintf("error redacting token from string: %s", compileErr)
		return newErrStr
	}

	redacted := re.ReplaceAllString(log, "*****")

	return redacted
}
