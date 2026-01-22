package domain

type MerchantProfile struct {
	Ucp VersionInfo `json:"ucp"`
}

type VersionInfo struct {
	Version string `json:"version"`
}

type PlatformProfile struct {
	Ucp         VersionInfo      `json:"ucp"`
	SigningKeys []JWK            `json:"signing_keys"`
	Services    PlatformServices `json:"services"`
}

type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	Kid string `json:"kid,omitempty"`
	Use string `json:"use,omitempty"`
	Alg string `json:"alg,omitempty"`
}

type PlatformServices struct {
	Platform PlatformService `json:"dev.ucp.platform"`
}

type PlatformService struct {
	Profile PlatformProfileEndpoint `json:"profile"`
}

type PlatformProfileEndpoint struct {
	URL string `json:"url"`
}
