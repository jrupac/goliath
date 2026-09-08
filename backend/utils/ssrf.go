package utils

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	log "github.com/golang/glog"
)

// ErrBlockedAddress is returned when a dial is refused for naming an address
// outside the public internet.
var ErrBlockedAddress = errors.New("address is not permitted")

// AddressAllowlist names addresses a guarded transport may reach despite not
// being on the public internet.
//
// It exists for a service deliberately run alongside this one — a bridge that
// manufactures feeds for sites publishing none — which from the guard's point
// of view is indistinguishable from the internal services the guard is there
// to protect. Empty by default: an allowlist is something an operator opts
// into for a service they run.
type AddressAllowlist map[string]bool

// NewAddressAllowlist resolves "host:port" entries into the concrete addresses
// a dial would reach.
//
// The guard compares against the address actually being dialed rather than the
// name asked for, so the allowlist has to be expressed the same way. Resolving
// here rather than at dial time also means a name cannot be repointed at some
// other internal address after the process starts: a host whose address
// changes simply stops being allowed, which is the safe direction to fail.
func NewAddressAllowlist(entries []string) (AddressAllowlist, error) {
	allowed := AddressAllowlist{}
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		host, port, err := net.SplitHostPort(entry)
		if err != nil {
			return nil, fmt.Errorf("allowlist entry %q is not host:port: %w", entry, err)
		}
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, fmt.Errorf("allowlist entry %q: %w", entry, err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("allowlist entry %q resolved to no addresses", entry)
		}
		for _, ip := range ips {
			allowed[net.JoinHostPort(ip.String(), port)] = true
		}
	}
	return allowed, nil
}

// Permits reports whether an address being dialed is on the allowlist.
func (a AddressAllowlist) Permits(address string) bool {
	return a[address]
}

// GuardedTransport returns a transport that refuses to connect to addresses
// outside the public internet, for fetching a URL this process did not choose.
// Addresses on `allowed` are reached anyway.
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
func GuardedTransport(subject string, timeout time.Duration, allowed AddressAllowlist) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			if allowed.Permits(address) {
				log.V(2).Infof("%s allowed configured address %s", subject, address)
				return nil
			}
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
