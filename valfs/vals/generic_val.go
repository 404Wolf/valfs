package valfs

import (
	"context"
)

// Val defines the required methods for a mutatable val object
type Val interface {
	Update(ctx context.Context) error
	Get(ctx context.Context) error
	List(ctx context.Context) error

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
	GetAuthor() Author
	GetVersionsLink() string
	GetModuleLink() string
	GetEndpointLink() string
}

// Author defines required methods for a static author object
type Author interface {
	GetId() int32
	GetUsername() string
}

type BasicAuthor struct {
	Id       int32
	Username string
}

func (a BasicAuthor) GetId() int32        { return a.Id }
func (a BasicAuthor) GetUsername() string { return a.Username }
