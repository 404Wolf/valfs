package valfs

import (
	"context"
)

const ApiPageLimit = 99

// Val defines the required methods for a mutatable val object
type Val interface {
	Update(ctx context.Context) error
	Load(ctx context.Context) error

	SetName(name string)
	SetValType(valType string)
	SetCode(code string)
	SetPrivacy(privacy string)
	SetReadme(readme string)

	GetId() string
	GetName() string
	GetValType() ValType
	GetCode() string
	GetPrivacy() string
	GetReadme() string
	GetVersion() int32
	GetVersionsLink() string
	GetModuleLink() string
	GetEndpointLink() string

	GetAuthorName() string
	GetAuthorId() string
}
