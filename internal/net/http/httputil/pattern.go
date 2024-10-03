package httputil

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"golang.org/x/net/http/httpguts"
)

// parsePattern parses a string into a Pattern.
// The string's syntax is
//
//	[METHOD] [HOST]/[PATH]
//
// where:
//   - METHOD is an HTTP method
//   - HOST is a hostname
//   - PATH consists of slash-separated segments, where each segment is either
//     a literal or a wildcard of the form "{name}", "{name...}", or "{$}".
//
// METHOD, HOST and PATH are all optional; that is, the string can be "/".
// If METHOD is present, it must be followed by at least one space or tab.
// Wildcard names must be valid Go identifiers.
// The "{$}" and "{name...}" wildcard must occur at the end of PATH.
// PATH may end with a '/'.
// Wildcard names in a path must be distinct.
func ParsePattern(s string) (method, host, path string, err error) {
	if len(s) == 0 {
		return "", "", "", errors.New("empty pattern")
	}
	off := 0 // offset into string
	defer func() {
		if err != nil {
			err = fmt.Errorf("at offset %d: %w", off, err)
		}
	}()

	method, rest, found := s, "", false
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		method, rest, found = s[:i], strings.TrimLeft(s[i+1:], " \t"), true
	}
	if !found {
		rest = method
		method = ""
	}
	if method != "" && !validMethod(method) {
		return "", "", "", fmt.Errorf("invalid method %q", method)
	}

	if found {
		off = len(method) + 1
	}
	i := strings.IndexByte(rest, '/')
	if i < 0 {
		return "", "", "", errors.New("host/path missing /")
	}
	host = rest[:i]
	rest = rest[i:]
	if j := strings.IndexByte(host, '{'); j >= 0 {
		off += j
		return "", "", "", errors.New("host contains '{' (missing initial '/'?)")
	}

	// An unclean path with a method that is not CONNECT can never match,
	// because paths are cleaned before matching.
	if method != "" && method != "CONNECT" && rest != cleanPath(rest) {
		return "", "", "", errors.New("non-CONNECT pattern with unclean path can never match")
	}

	return method, host, rest, nil
}

func validMethod(method string) bool {
	/*
	     Method         = "OPTIONS"                ; Section 9.2
	                    | "GET"                    ; Section 9.3
	                    | "HEAD"                   ; Section 9.4
	                    | "POST"                   ; Section 9.5
	                    | "PUT"                    ; Section 9.6
	                    | "DELETE"                 ; Section 9.7
	                    | "TRACE"                  ; Section 9.8
	                    | "CONNECT"                ; Section 9.9
	                    | extension-method
	   extension-method = token
	     token          = 1*<any CHAR except CTLs or separators>
	*/
	return len(method) > 0 && strings.IndexFunc(method, isNotToken) == -1
}

func isNotToken(r rune) bool {
	return !httpguts.IsTokenRune(r)
}

// cleanPath returns the canonical path for p, eliminating . and .. elements.
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		// Fast path for common case of p being the string we want:
		if len(p) == len(np)+1 && strings.HasPrefix(p, np) {
			np = p
		} else {
			np += "/"
		}
	}
	return np
}
