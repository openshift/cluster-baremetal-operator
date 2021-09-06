package network

import (
	"net"
	"reflect"
	"testing"
)

func TestIPOption(t *testing.T) {
	tests := []struct {
		ns   NetworkStackType
		want string
	}{
		{
			ns:   NetworkStackV4,
			want: "ip=dhcp",
		},
		{
			ns:   NetworkStackV6,
			want: "ip=dhcp6",
		},
		{
			ns:   NetworkStackDual,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.ns.IPOption(); got != tt.want {
				t.Errorf("IPOption() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNetworkStack(t *testing.T) {
	tests := []struct {
		name    string
		ips     []net.IP
		want    NetworkStackType
		wantErr bool
	}{
		{
			name: "v4 basic",
			ips:  []net.IP{net.ParseIP("192.168.0.1")},
			want: NetworkStackV4,
		},
		{
			name: "v4 in v6 format: basic",
			ips:  []net.IP{net.ParseIP("::FFFF:192.168.0.1")},
			want: NetworkStackV4,
		},
		{
			name: "v6: basic",
			ips:  []net.IP{net.ParseIP("2001:db8::68")},
			want: NetworkStackV6,
		},
		{
			name: "dual: basic",
			ips:  []net.IP{net.ParseIP("2001:db8::68"), net.ParseIP("192.168.0.1")},
			want: NetworkStackDual,
		},
		{
			name: "v6: with v4 local",
			ips:  []net.IP{net.ParseIP("2001:db8::68"), net.ParseIP("127.0.0.1")},
			want: NetworkStackV6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := networkStack(tt.ips)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("networkStack() = %v, want %v", got, tt.want)
			}
		})
	}
}
