package server

type errBoom struct{}

func (errBoom) Error() string { return "boom" }
