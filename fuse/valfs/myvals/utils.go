package fuse

import (
	valfile "github.com/404wolf/valfs/fuse/valfs/myvals/valfile"
	"regexp"
)

func GuessFilename(guess string) (
	hopeless bool,
	valName *string,
	valType *valfile.ValType,
) {
	// Parse the filename of the val
	valNameAttempt, valTypeAttempt := valfile.ExtractFromFilename(guess)

	// Try to guess the type if it is unknown
	if valTypeAttempt == valfile.Unknown {
		re := regexp.MustCompile(`^([^\.]+\.?)+\.tsx?`)
		if re.MatchString(guess) {
			valName = &re.FindStringSubmatch(guess)[1]
			valTypeRef := valfile.DefaultType
			return false, valName, &valTypeRef
		} else {
			return true, nil, nil
		}
	} else {
		return false, &valNameAttempt, &valTypeAttempt
	}
}
