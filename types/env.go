package types

//---------- Env ---------

// Env defines the state of the blockchain environment this contract is
// running in. This must contain only trusted data - nothing from the Tx itself
// that has not been verfied (like Signer).
//
// Env are json encoded to a byte slice before passing to the wasm contract.
type Env struct {
	Block    BlockInfo    `json:"block"`
	Contract ContractInfo `json:"contract"`
}

type BlockInfo struct {
	// block height this transaction is executed
	Height uint64 `json:"height"`
	// time in seconds since unix epoch (since CosmWasm 0.3)
	Time uint64 `json:"time"`
	// Nanoseconds of the block time (since CosmWasm 0.11)
	TimeNanos uint64 `json:"time_nanos"`
	ChainID   string `json:"chain_id"`
}

type MessageInfo struct {
	// binary encoding of sdk.AccAddress executing the contract
	Sender HumanAddress `json:"sender"`
	// amount of funds send to the contract along with this message
	Funds Coins `json:"funds"`
}

type ContractInfo struct {
	// binary encoding of sdk.AccAddress of the contract, to be used when sending messages
	Address HumanAddress `json:"address"`
}
