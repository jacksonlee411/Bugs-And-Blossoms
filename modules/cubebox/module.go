package cubebox

// NewDefaultStore is the module composition root for the CubeBox persistence adapter.
func NewDefaultStore(pool TxBeginner) *Store {
	return NewStore(pool)
}

// NewDefaultGatewayService wires the default in-process runtime to the provider gateway.
func NewDefaultGatewayService(configReader RuntimeConfigReader, adapter ProviderAdapter, secretResolver SecretResolver) *GatewayService {
	return NewGatewayService(NewRuntime(), configReader, adapter, secretResolver)
}
