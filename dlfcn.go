package purego

// Dlerror represents an error value returned from Dlopen, Dlsym, or Dlclose.
type Dlerror struct {
	s string
}

func (e Dlerror) Error() string {
	return e.s
}
