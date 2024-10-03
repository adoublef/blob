package http

import (
	"fmt"
	"io"
	"net/http"
)

type statusHandler struct {
	code int
	s    string
}

func (sh statusHandler) Err() error {
	if sh.code < 400 {
		return nil
	}
	s := http.StatusText(sh.code)
	return fmt.Errorf("%d %s: %s", sh.code, s, sh.s[:20])
}

func (sh statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// carry [context.Context] throughout the lifetime of this handler?
	if err := sh.Err(); err != nil {
		// log the error if it is
		http.Error(w, "", sh.code)
		return
	}
	// also handle redirects if applicable
	if sc := sh.code; sc > 299 && sc < 400 {
		// s is assumed to be a url
		http.Redirect(w, r, sh.s, sh.code)
		return
	}
	// dont really care about [sh.s]
	w.WriteHeader(sh.code)
	io.WriteString(w, http.StatusText(sh.code))
}
