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
	return fmt.Errorf("%d %s: %s", sh.code, sh.StatusText(), sh.s[:20])
}

func (sh statusHandler) StatusText() string {
	return http.StatusText(sh.code)
}

func (sh statusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// carry [context.Context] throughout the lifetime of this handler?
	if err := sh.Err(); err != nil {
		// log the error if it is
		http.Error(w, sh.StatusText(), sh.code)
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
	io.WriteString(w, sh.StatusText())
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
	s := "The server was unable to complete your request. Please try again later."
	http.Error(w, s, http.StatusInternalServerError)
}
