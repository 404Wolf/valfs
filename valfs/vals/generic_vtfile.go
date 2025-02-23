package valfs

import (
	"context"
	"fmt"
	"strings"
	"time"

	_ "embed"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type VTFileType string

const ApiPageLimit = 99

const (
	VTFileTypeDirectory VTFileType = "directory"
	VTFileTypeFile      VTFileType = "file"
	VTFileTypeInterval  VTFileType = "interval"
	VTFileTypeHTTP      VTFileType = "http"
	VTFileTypeEmail     VTFileType = "email"
	VTFileTypeScript    VTFileType = "script"
	VTFileTypeUnknown   VTFileType = "unknown"
)

// VTFile defines the required methods for a mutatable val
// town file object. Files can be vals or project files, but both must
// implement VTFile.
type VTFile interface {
	Save(ctx context.Context) error
	Load(ctx context.Context) error

	GetId() string
	GetContainer() *VTFileContainer
	GetPath() string
	GetType() VTFileType

	SetPath(path string)
	SetType(type_ string)

	GetApiUrl() string
	GetModuleUrl() string
	GetDeployedUrl() *string
}

// ValVTFile defines the interface for Val Town file operations and properties
type ValVTFile interface {
	VTFile

	GetAsPackedText() (*string, error)
	UpdateFromPackedText(context context.Context, text string) error

	GetName() string
	GetCode() string
	GetPrivacy() string
	GetReadme() string
	GetVersion() int32
	GetAuthorName() string
	GetAuthorId() string
	GetCreatedAt() time.Time
	GetUrl() string
	GetLikeCount() int32
	GetReferenceCount() int32
	GetVersionsLink() string
	GetModuleLink() string
	GetEndpointLink() string
	GetInode() *fs.Inode

	SetName(name string)
	SetValType(valType string)
	SetCode(code string)
	SetPrivacy(privacy string)
	SetReadme(readme string)
}

const (
	Unlisted = "unlisted"
	Public   = "public"
	Private  = "private"
)

var abbreviate = map[VTFileType]string{
	VTFileTypeDirectory: "U",
	VTFileTypeHTTP:      "H",
	VTFileTypeScript:    "S",
	VTFileTypeInterval:  "C",
	VTFileTypeEmail:     "E",
}

var unabbreviate = map[string]VTFileType{
	"U":        VTFileTypeDirectory,
	"H":        VTFileTypeHTTP,
	"S":        VTFileTypeScript,
	"C":        VTFileTypeInterval,
	"E":        VTFileTypeEmail,
	"http":     VTFileTypeHTTP,
	"script":   VTFileTypeScript,
	"interval": VTFileTypeInterval,
	"email":    VTFileTypeEmail,
}

const ValExtension = "tsx"
const DefaultValPrivacy = Unlisted
const DefaultType = VTFileTypeScript

//go:embed vals_templates/script.ts
var scriptTemplate []byte

//go:embed vals_templates/email.ts
var emailTemplate []byte

//go:embed vals_templates/http.ts
var httpTemplate []byte

//go:embed vals_templates/cron.ts
var cronTemplate []byte

// GetTemplate returns the template (base) for a given VTFileType
func GetTemplate(fileType VTFileType) string {
	switch fileType {
	case VTFileTypeEmail:
		return string(emailTemplate)
	case VTFileTypeInterval:
		return string(cronTemplate)
	case VTFileTypeScript:
		return string(scriptTemplate)
	case VTFileTypeHTTP:
		return string(httpTemplate)
	}
	return ""
}

// ExtractFromValFilename takes a filename and returns the corresponding name
// and VTFileType
func ExtractFromValFilename(filename string) (string, VTFileType) {
	parts := strings.Split(filename, ".")
	if len(parts) < 3 {
		return filename, VTFileTypeDirectory
	}

	// Extract the name (everything before the last two parts)
	name := strings.Join(parts[:len(parts)-2], ".")

	// Determine the type
	typeAbbrev := parts[len(parts)-2]
	fileType := unabbreviate[typeAbbrev]

	return name, fileType
}

// ConstructValFilename takes a base name and VTFileType and returns the
// corresponding filename
func ConstructValFilename(baseName string, fileType VTFileType) string {
	if fileType == VTFileTypeDirectory {
		return fmt.Sprintf("%s.%s", baseName, ValExtension)
	}

	return fmt.Sprintf("%s.%s.tsx", baseName, abbreviate[fileType])
}

var ValFileMeta = fs.StableAttr{Mode: fuse.S_IFREG | 0777}
