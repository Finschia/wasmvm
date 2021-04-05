package types

import "encoding/json"

// SubMsg wraps a CosmosMsg with some metadata for handling replies (ID) and optionally
// limiting the gas usage (GasLimit)
type SubMsg struct {
	ID       uint64    `json:"id"`
	Msg      CosmosMsg `json:"msg"`
	GasLimit *uint64   `json:"gas_limit,omitempty"`
}

type Reply struct {
	ID     uint64        `json:"id"`
	Result SubcallResult `json:"result"`
}

// SubcallResult is the raw response we return from the sdk -> reply after executing a SubMsg.
// This is mirrors Rust's ContractResult<SubcallResponse>.
type SubcallResult struct {
	Ok  *SubcallResponse `json:"ok,omitempty"`
	Err string           `json:"error,omitempty"`
}

type SubcallResponse struct {
	Events Events `json:"events"`
	Data   []byte `json:"data,omitempty"`
}

// Events must encode empty array as []
type Events []Event

// MarshalJSON ensures that we get [] for empty arrays
func (e Events) MarshalJSON() ([]byte, error) {
	if len(e) == 0 {
		return []byte("[]"), nil
	}
	var raw []Event = e
	return json.Marshal(raw)
}

// UnmarshalJSON ensures that we get [] for empty arrays
func (e *Events) UnmarshalJSON(data []byte) error {
	// make sure we deserialize [] back to null
	if string(data) == "[]" || string(data) == "null" {
		return nil
	}
	var raw []Event
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*e = raw
	return nil
}

type Event struct {
	Type       string          `json:"type"`
	Attributes EventAttributes `json:"attributes"`
}
