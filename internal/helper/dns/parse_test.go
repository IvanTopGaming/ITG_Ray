package dns

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDNSAddresses_EnglishLocale(t *testing.T) {
	out := parseDNSAddresses(`Configuration for interface "Ethernet"
    DNS servers configured through DHCP:  192.168.2.1
                                          1.1.1.1
    Register with which suffix:           Primary only
`)
	require.Equal(t, []string{"192.168.2.1", "1.1.1.1"}, out)
}

func TestParseDNSAddresses_RussianLocale(t *testing.T) {
	out := parseDNSAddresses("Конфигурация интерфейса \"Ethernet\"\n    DNS-серверы с настройкой через DHCP:  192.168.2.1\n                                            1.0.0.1\n")
	require.Equal(t, []string{"192.168.2.1", "1.0.0.1"}, out)
}

func TestParseDNSAddresses_RejectsInvalidOctets(t *testing.T) {
	// Octets > 255 are extracted by the regex but rejected by looksLikeValidIP.
	out := parseDNSAddresses("DNS servers:  999.999.999.999\n   1.1.1.1\n")
	require.Equal(t, []string{"1.1.1.1"}, out)
}

func TestParseDNSAddresses_NoIPs(t *testing.T) {
	require.Empty(t, parseDNSAddresses(`Configuration for interface "Ethernet"
    Statically configured DNS servers:  None
`))
}

func TestParseDNSAddresses_MultipleOnOneLine(t *testing.T) {
	// Defensive: regex picks up multiple IPs per line if netsh ever emits them.
	out := parseDNSAddresses("    DNS:  1.1.1.1, 8.8.8.8\n")
	require.Equal(t, []string{"1.1.1.1", "8.8.8.8"}, out)
}
