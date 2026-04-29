package callerfixture

import (
	"errors"
	"strings"
)

var ErrSentinel = errors.New("sentinel")

type Service struct {
	name string
}

func (s *Service) Validate(input string) error {
	if input == "" {
		return errors.New("empty input")
	}

	normalize := func(in string) string {
		return strings.ToUpper(in)
	}
	if normalize(input) == "BAD" {
		return errors.New("bad input")
	}
	return nil
}
