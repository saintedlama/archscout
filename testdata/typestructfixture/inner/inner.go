package inner

type Base struct {
	ID int
}

// SessionID and Event are used by the parent package's User struct to
// exercise composite-type field rendering (map keys, channel elements).
type SessionID string
type Event struct{ Code int }

// Pinger is an interface embedded into the parent package's Service
// interface. Method-set resolution must surface "Pinger" (or its qname) in
// Service's Embeds slice, while Service.Methods only lists the directly
// declared methods.
type Pinger interface {
	Ping() error
}
