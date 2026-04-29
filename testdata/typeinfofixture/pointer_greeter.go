package main

type PointerGreeter struct {
	prefix string
}

func (p *PointerGreeter) Greet(name string) string {
	return p.prefix + " " + name
}
