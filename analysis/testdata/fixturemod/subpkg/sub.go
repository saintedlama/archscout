package subpkg

import "fmt"

func SubErr() error {
	return fmt.Errorf("sub error")
}
