//go:build !windows

package main

// serverExcludeForTUN returns the CIDRs handed to the sing-box TUN inbound's
// route_exclude_address, i.e. destinations the auto_route machinery must keep
// OUT of the tunnel and leave on the host's physical routing table.
//
// Two distinct reasons to exclude:
//
//  1. The VPN server's own IP (ip/32) — otherwise the entry-node handshake
//     would loop back through the proxied path.
//
//  2. RFC1918 + IPv4 link-local — the LAN. auto_route installs split-default
//     routes covering all of IPv4 (0.0.0.0/2, 64.0.0.0/4, …), which also
//     swallow the private ranges. That silently breaks *incoming* LAN
//     connections: a reply packet from a host-local service (sshd, samba, a
//     web UI) carries src=<LAN ip of the physical NIC>, but the route lookup
//     sends it into the TUN, where sing-box's gvisor stack drops it because
//     src isn't the tun interface address (198.18.0.1). The client never gets
//     the handshake reply → timeout/reset. The route-rule "private IPs →
//     direct" cannot help: that decision only applies to connections sing-box
//     itself accepts, whereas this reply belongs to a connection established
//     entirely outside sing-box. The fix has to be at the OS-route level, here.
//
// IPv6 is deliberately NOT excluded here. The TUN is dual-stack and FakeIP
// hands out synthetic v6 addresses from fc00::/18; excluding the ULA range
// fc00::/7 (which contains fc00::/18) would break domain-based v6 proxying.
// The reported breakage is IPv4-only, so v6 LAN exclusion is left out rather
// than risk the FakeIP overlap.
func serverExcludeForTUN(ip string) []string {
	return []string{
		ip + "/32",
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // IPv4 link-local (APIPA/mDNS)
	}
}
