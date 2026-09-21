package subscription

import "encoding/json"

type xrayDocument struct {
	Remarks   string            `json:"remarks"`
	Outbounds []json.RawMessage `json:"outbounds"`
}

type xrayOutbound struct {
	Protocol      string          `json:"protocol"`
	Tag           string          `json:"tag"`
	Settings      json.RawMessage `json:"settings"`
	Stream        xrayStream      `json:"streamSettings"`
	ProxySettings struct {
		Tag string `json:"tag"`
	} `json:"proxySettings"`
}

type xrayUser struct {
	ID         string          `json:"id"`
	Encryption string          `json:"encryption"`
	Flow       string          `json:"flow"`
	Reverse    json.RawMessage `json:"reverse"`
}

type xrayEndpoint struct {
	Address string     `json:"address"`
	Port    int        `json:"port"`
	Users   []xrayUser `json:"users"`
}

type xraySettings struct {
	xrayUser
	Address string         `json:"address"`
	Port    int            `json:"port"`
	VNext   []xrayEndpoint `json:"vnext"`
}

type xrayStream struct {
	Network     string      `json:"network"`
	Security    string      `json:"security"`
	Address     string      `json:"address"`
	Port        int         `json:"port"`
	TLS         xrayTLS     `json:"tlsSettings"`
	Reality     xrayReality `json:"realitySettings"`
	TCP         *xrayTCP    `json:"tcpSettings"`
	Raw         *xrayTCP    `json:"rawSettings"`
	WS          xrayHTTP    `json:"wsSettings"`
	HTTPUpgrade xrayHTTP    `json:"httpupgradeSettings"`
	XHTTP       *xrayHTTP   `json:"xhttpSettings"`
	SplitHTTP   *xrayHTTP   `json:"splithttpSettings"`
	GRPC        struct {
		ServiceName string `json:"serviceName"`
		Authority   string `json:"authority"`
		MultiMode   bool   `json:"multiMode"`
	} `json:"grpcSettings"`
	KCP struct {
		Seed   string `json:"seed"`
		Header struct {
			Type string `json:"type"`
		} `json:"header"`
	} `json:"kcpSettings"`
	QUIC struct {
		Security string `json:"security"`
		Key      string `json:"key"`
		Header   struct {
			Type string `json:"type"`
		} `json:"header"`
	} `json:"quicSettings"`
	Sockopt struct {
		DialerProxy string `json:"dialerProxy"`
	} `json:"sockopt"`
}

type xrayTLS struct {
	ServerName           string          `json:"serverName"`
	Fingerprint          string          `json:"fingerprint"`
	ALPN                 []string        `json:"alpn"`
	AllowInsecure        bool            `json:"allowInsecure"`
	Certificates         json.RawMessage `json:"certificates"`
	PinnedPeerCertSHA256 string          `json:"pinnedPeerCertSha256"`
	VerifyPeerCertByName string          `json:"verifyPeerCertByName"`
	ECHConfigList        string          `json:"echConfigList"`
}

type xrayReality struct {
	ServerName    string `json:"serverName"`
	Fingerprint   string `json:"fingerprint"`
	PublicKey     string `json:"publicKey"`
	Password      string `json:"password"`
	ShortID       string `json:"shortId"`
	SpiderX       string `json:"spiderX"`
	Mldsa65Verify string `json:"mldsa65Verify"`
}

type xrayTCP struct {
	Header struct {
		Type    string          `json:"type"`
		Request json.RawMessage `json:"request"`
	} `json:"header"`
}

type xrayHTTP struct {
	XPaddingKey         string            `json:"xPaddingKey"`
	XPaddingHeader      string            `json:"xPaddingHeader"`
	XPaddingPlacement   string            `json:"xPaddingPlacement"`
	XPaddingMethod      string            `json:"xPaddingMethod"`
	UplinkHTTPMethod    string            `json:"uplinkHTTPMethod"`
	SessionPlacement    string            `json:"sessionPlacement"`
	SessionKey          string            `json:"sessionKey"`
	SeqPlacement        string            `json:"seqPlacement"`
	SeqKey              string            `json:"seqKey"`
	UplinkDataPlacement string            `json:"uplinkDataPlacement"`
	UplinkDataKey       string            `json:"uplinkDataKey"`
	XPaddingObfsMode    bool              `json:"xPaddingObfsMode"`
	NoGRPCHeader        bool              `json:"noGRPCHeader"`
	NoSSEHeader         bool              `json:"noSSEHeader"`
	Host                string            `json:"host"`
	Path                string            `json:"path"`
	Mode                string            `json:"mode"`
	Headers             map[string]string `json:"headers"`
	DownloadSettings    json.RawMessage   `json:"downloadSettings"`
	Extra               json.RawMessage   `json:"extra"`
}
