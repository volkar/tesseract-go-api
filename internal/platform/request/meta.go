package request

import (
	"net"
	"net/http"
)

type Metadata struct {
	IP        string
	UserAgent string
	Location  string
}

/* Get metadata from request */
func GetMetaFromRequest(r *http.Request) Metadata {
	// IP
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	// Location can be extracted from IP by something like MaxMind GeoLite2
	// Use IP as location for simplicity

	return Metadata{
		IP:        ip,
		UserAgent: r.UserAgent(),
		Location:  ip,
	}
}
