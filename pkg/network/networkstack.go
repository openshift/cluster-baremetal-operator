package network

import (
	"net"
	"net/url"

	"github.com/pkg/errors"
)

type NetworkStackType int

const (
	NetworkStackV4   NetworkStackType = 1 << iota
	NetworkStackV6   NetworkStackType = 1 << iota
	NetworkStackDual NetworkStackType = (NetworkStackV4 | NetworkStackV6)
)

// NetworkStackFromURL given a url, lookup the host and return NetworkStackType
func NetworkStackFromURL(input string) (NetworkStackType, error) {
	if input == "" {
		return NetworkStackV4, errors.New("can not calculate NetworkStackType from empty url")
	}

	inputURL, err := url.Parse(input)
	if err != nil {
		return NetworkStackV4, errors.Wrapf(err, "unable to parse URL %s", input)
	}

	host, _, err := net.SplitHostPort(inputURL.Host)
	if err != nil {
		return NetworkStackV4, err
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return NetworkStackV4, errors.Wrap(err, "could not lookupIP for internal APIServer: "+host)
	}

	return networkStack(ips), nil
}

func (ns NetworkStackType) IPOption() string {
	var optionValue string
	switch ns {
	case NetworkStackV4:
		optionValue = "ip=dhcp"
	case NetworkStackV6:
		optionValue = "ip=dhcp6"
	case NetworkStackDual:
		optionValue = ""
	}
	return optionValue
}

func networkStack(ips []net.IP) NetworkStackType {
	ns := NetworkStackType(0)
	for _, ip := range ips {
		if ip.IsLoopback() {
			continue
		}
		if ip.To4() != nil {
			ns |= NetworkStackV4
		} else {
			ns |= NetworkStackV6
		}
	}
	return ns
}
