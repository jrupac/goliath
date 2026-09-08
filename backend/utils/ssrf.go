package utils

import (
	"context"
	"errors"
	"net"
	"net/http"
	"syscall"
	"time"

	log "github.com/golang/glog"
)

// ErrBlockedAddress is returned when a dial is refused for naming an address
// outside the public internet.
var ErrBlockedAddress = errors.New("address is not permitted")

// GuardedTransport returns a transport that refuses to connect to addresses
// outside the public internet, for fetching a URL this process did not choose.
//
// The check runs in Control, which is called after the name has been resolved
// and with the address actually about to be dialed. Validating the hostname
// instead would miss a name that resolves to a private address, and would miss
// it again if a second lookup returned something different. Redirects are
// dialed through the same guard, so a redirect cannot reach anywhere the
// original URL could not.
//
// `subject` names the caller in the log line, since a refusal is worth being
// able to attribute.
func GuardedTransport(subject string, timeout time.Duration) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if port != "80" && port != "443" {
				log.Warningf("%s refused port %s", subject, port)
				return ErrBlockedAddress
			}
			ip := net.ParseIP(host)
			if ip == nil || !IsPublicAddress(ip) {
				log.Warningf("%s refused address %s", subject, host)
				return ErrBlockedAddress
			}
			return nil
		},
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, addr)
	}
	return transport
}

// IsPublicAddress reports whether an address is one this process may reach on
// behalf of a URL it did not choose.
//
// Everything that is not routable on the public internet is refused, because
// the value of reaching it is that the server can and the requester cannot:
// loopback and private ranges are the internal services this process sits
// alongside, and link-local covers the address cloud providers answer instance
// credentials on.
func IsPublicAddress(ip net.IP) bool {
	return !ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsUnspecified() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsInterfaceLocalMulticast() &&
		!ip.IsMulticast()
}
