package api

type Greeter interface {
	Greet(name string) string
}

type Service struct{}

func (s *Service) Run() string {
	return "running"
}

// Describe is called via a value receiver.
func (s Service) Describe() string {
	return "describing"
}
