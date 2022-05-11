package provisioning

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetworkStackIpOption(t *testing.T) {
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
			want: "ip=dhcp,dhcp6",
		},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.ns.IpOption())
		})
	}
}
