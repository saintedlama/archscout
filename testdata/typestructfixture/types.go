package typestructfixture

import (
	"io"

	"example.com/typestructfixture/inner"
)

type User struct {
	inner.Base

	Name      string `json:"name"`
	Email     string `json:"email,omitempty"`
	Age, Year int

	// Composite-type fields exercise the typeText path. Without it, every
	// field below would surface with an empty TypeName because exprText
	// only handles Ident/StarExpr/SelectorExpr.
	Roles    []string
	Sessions map[inner.SessionID]*inner.Base
	Notify   chan inner.Event
	Hook     func(in string) (string, error)
}

type Service interface {
	io.Reader
	inner.Pinger

	Run() error
	Stop(force bool) error
}
