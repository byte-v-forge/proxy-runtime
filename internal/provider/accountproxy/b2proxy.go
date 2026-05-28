package accountproxy

func init() {
	RegisterDefinition(Definition{ProviderID: ProviderB2Proxy, DisplayName: "B2Proxy", DefaultProtocol: "socks5", Protocols: []string{"http", "socks5"}, BuildUsername: passthroughUsername})
}
