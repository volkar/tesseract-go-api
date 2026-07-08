package request

import (
	"net"
	"net/http"
	"strconv"

	"github.com/mileusna/useragent"
)

type Metadata struct {
	IP        string
	UserAgent string
	Device    string
	Os        string
	Browser   string
	Location  string
}

/* Get metadata from request */
func GetMetaFromRequest(r *http.Request) Metadata {
	ua := useragent.Parse(r.UserAgent())
	// IP
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	// OS
	os := ua.OS
	if os != "" && ua.OSVersion != "" {
		os += " " + ua.OSVersion
	}
	// Browser
	b := ua.Name
	if b != "" && ua.VersionNo.Major != 0 {
		b += " " + strconv.Itoa(ua.VersionNo.Major)
	}
	// Device
	d := ua.Device

	// Location can be extracted from IP by something like MaxMind GeoLite2
	// Use IP as location for simplicity
	l := ip

	return Metadata{
		IP:        ip,
		UserAgent: r.UserAgent(),
		Device:    d,
		Os:        os,
		Browser:   b,
		Location:  l,
	}
}
