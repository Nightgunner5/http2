package http2

import (
	"net/http"
	"strings"
	"time"
)

func splitETags(header string, weak bool, actual string) (tags []string) {
	// Special case: "If-None-Match: *"
	if header == "*" {
		tags = []string{actual}
		return
	}

	for _, tag := range strings.FieldsFunc(header, func(r rune) bool {
		return r == ','
	}) {
		tag = strings.TrimSpace(tag)
		if weak && strings.HasPrefix(tag, "W/\"") && strings.HasSuffix(tag, "\"") {
			tags = append(tags, tag[3:len(tag)-1])
		} else if strings.HasPrefix(tag, "\"") && strings.HasSuffix(tag, "\"") {
			tags = append(tags, tag[1:len(tag)-1])
		}
	}
	return
}

// Inserts an ETag header and checks the If-None-Match header (defined in
// RFC 2616). Returns true if the request matched the given etag. The etag
// is "weak" if it does not change every time the bytes of the page change,
// for example, a page that contains a representation of the time taken to
// generate it would not have much success if the cache was invalidated on
// every pageload.
//
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.26
//
// Usage:
//       // At the top of your handler:
//       if CheckETag(etag, false, w, r) {
//           return
//       }
//       // Output page content as normal
func CheckETag(etag string, weak bool, w http.ResponseWriter, r *http.Request) bool {
	if weak {
		w.Header().Set("ETag", "W/\""+etag+"\"")
	} else {
		w.Header().Set("ETag", "\""+etag+"\"")
	}

	for _, tag := range splitETags(r.Header.Get("If-None-Match"), weak, etag) {
		if tag == etag {
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}
	return false
}

// Inserts a Last-Modified header and checks the If-Modified-Since header
// (defined in RFC 2616). Returns true if the time is valid and is not before
// the given lastModified time.
//
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.25
//
// Usage:
//       // At the top of your handler:
//       if CheckLastModified(lastModified, w, r) {
//           return
//       }
//       // Output page content as normal
func CheckLastModified(lastModified time.Time, w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
	if t, err := time.Parse(http.TimeFormat, r.Header.Get("If-Modified-Since")); err != nil || lastModified.After(t) {
		return false
	}
	w.WriteHeader(http.StatusNotModified)
	return true
}
