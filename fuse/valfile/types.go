package fuse

import (
	"fmt"
	"strings"

	_ "embed"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// Represents the type of val
type ValType string

const (
	Unknown ValType = "unknown"
	HTTP    ValType = "http"
	Script  ValType = "script"
	Cron    ValType = "cron"
	Email   ValType = "email"
)

const (
	Unlisted = "unlisted"
	Public   = "public"
	Private  = "private"
)

var abbreviate = map[ValType]string{
	Unknown: "U",
	HTTP:    "H",
	Script:  "S",
	Cron:    "C",
	Email:   "E",
}

var unabbreviate = map[string]ValType{
	"U": Unknown,
	"H": HTTP,
	"S": Script,
	"C": Cron,
	"E": Email,
}

const ValExtension = "tsx"
const DefaultPrivacy = Unlisted
const DefaultType = Script

//go:embed templates/script.ts
var scriptTemplate []byte

//go:embed templates/email.ts
var emailTemplate []byte

//go:embed templates/http.ts
var httpTemplate []byte

//go:embed templates/cron.ts
var cronTemplate []byte

func GetTemplate(valType ValType) string {
	switch valType {
	case Email:
		return string(emailTemplate)
	case Cron:
		return string(cronTemplate)
	case Script:
		return string(scriptTemplate)
	case HTTP:
		return string(httpTemplate)
	}
	return ""
}

// Takes a filename and returns the corresponding name and ValType
func ExtractFromFilename(filename string) (string, ValType) {
	parts := strings.Split(filename, ".")
	if len(parts) < 3 {
		return filename, Unknown
	}

	// Extract the name (everything before the last two parts)
	name := strings.Join(parts[:len(parts)-2], ".")

	// Determine the type
	valType := ValType(parts[len(parts)-2])
	valType = unabbreviate[string(valType)]

	return name, valType
}

// Takes a base name and ValType and returns the corresponding filename
func ConstructFilename(baseName string, valType ValType) string {
	if valType == Unknown {
		return fmt.Sprintf("%s.%s", baseName, ValExtension)
	}

	return fmt.Sprintf("%s.%s.tsx", baseName, abbreviate[valType])
}

var ValFileMeta = fs.StableAttr{Mode: fuse.S_IFREG | 0777}
