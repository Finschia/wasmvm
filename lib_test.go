package cosmwasm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/line/wasmvm/api"
	"github.com/line/wasmvm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	TESTING_FEATURES     = "staking,stargate,iterator"
	TESTING_PRINT_DEBUG  = false
	TESTING_GAS_LIMIT    = uint64(500_000_000_000) // ~0.5ms
	TESTING_MEMORY_LIMIT = 32                      // MiB
	TESTING_CACHE_SIZE   = 100                     // MiB
)

const HACKATOM_TEST_CONTRACT = "./api/testdata/hackatom.wasm"
const EVENTS_TEST_CONTRACT = "./api/testdata/events.wasm"

func withVM(t *testing.T) *VM {
	tmpdir, err := ioutil.TempDir("", "wasmvm-testing")
	require.NoError(t, err)
	vm, err := NewVM(tmpdir, TESTING_FEATURES, TESTING_MEMORY_LIMIT, TESTING_PRINT_DEBUG, TESTING_CACHE_SIZE)
	require.NoError(t, err)

	t.Cleanup(func() {
		vm.Cleanup()
		os.RemoveAll(tmpdir)
	})
	return vm
}

func createTestContract(t *testing.T, vm *VM, path string) Checksum {
	wasm, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	checksum, err := vm.Create(wasm)
	require.NoError(t, err)
	return checksum
}

func TestCreateAndGet(t *testing.T) {
	vm := withVM(t)

	wasm, err := ioutil.ReadFile(HACKATOM_TEST_CONTRACT)
	require.NoError(t, err)

	checksum, err := vm.Create(wasm)
	require.NoError(t, err)

	code, err := vm.GetCode(checksum)
	require.NoError(t, err)
	require.Equal(t, WasmCode(wasm), code)
}

func TestHappyPath(t *testing.T) {
	vm := withVM(t)
	checksum := createTestContract(t, vm, HACKATOM_TEST_CONTRACT)

	deserCost := types.UFraction{1, 1}
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Coins{types.NewCoin(250, "ATOM")}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	// instantiate
	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	ires, _, err := vm.Instantiate(checksum, env, info, msg, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// execute
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	env = api.MockEnv()
	info = api.MockInfo("fred", nil)
	hres, _, err := vm.Execute(checksum, env, info, []byte(`{"release":{}}`), store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.Equal(t, 1, len(hres.Messages))

	// make sure it read the balance properly and we got 250 atoms
	dispatch := hres.Messages[0].Msg
	require.NotNil(t, dispatch.Bank, "%#v", dispatch)
	require.NotNil(t, dispatch.Bank.Send, "%#v", dispatch)
	send := dispatch.Bank.Send
	assert.Equal(t, "bob", send.ToAddress)
	assert.Equal(t, balance, send.Amount)
	// check the data is properly formatted
	expectedData := []byte{0xF0, 0x0B, 0xAA}
	assert.Equal(t, expectedData, hres.Data)
}

func TestGetMetrics(t *testing.T) {
	vm := withVM(t)

	// GetMetrics 1
	metrics, err := vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{}, metrics)

	// Create contract
	checksum := createTestContract(t, vm, HACKATOM_TEST_CONTRACT)

	deserCost := types.UFraction{1, 1}

	// GetMetrics 2
	metrics, err = vm.GetMetrics()
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{}, metrics)

	// Instantiate 1
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Coins{types.NewCoin(250, "ATOM")}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	ires, _, err := vm.Instantiate(checksum, env, info, msg1, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// GetMetrics 3
	metrics, err = vm.GetMetrics()
	assert.NoError(t, err)
	require.Equal(t, uint32(0), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 5665691, metrics.SizeMemoryCache, 0.18)

	// Instantiate 2
	msg2 := []byte(`{"verifier": "fred", "beneficiary": "susi"}`)
	ires, _, err = vm.Instantiate(checksum, env, info, msg2, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// GetMetrics 4
	metrics, err = vm.GetMetrics()
	assert.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 5665691, metrics.SizeMemoryCache, 0.18)

	// Pin
	err = vm.Pin(checksum)
	require.NoError(t, err)

	// GetMetrics 5
	metrics, err = vm.GetMetrics()
	assert.NoError(t, err)
	require.Equal(t, uint32(2), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 5665691, metrics.SizePinnedMemoryCache, 0.18)
	require.InEpsilon(t, 5665691, metrics.SizeMemoryCache, 0.18)

	// Instantiate 3
	msg3 := []byte(`{"verifier": "fred", "beneficiary": "bert"}`)
	ires, _, err = vm.Instantiate(checksum, env, info, msg3, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// GetMetrics 6
	metrics, err = vm.GetMetrics()
	assert.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 5665691, metrics.SizePinnedMemoryCache, 0.18)
	require.InEpsilon(t, 5665691, metrics.SizeMemoryCache, 0.18)

	// Unpin
	err = vm.Unpin(checksum)
	require.NoError(t, err)

	// GetMetrics 7
	metrics, err = vm.GetMetrics()
	assert.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.Equal(t, uint64(0), metrics.SizePinnedMemoryCache)
	require.InEpsilon(t, 5665691, metrics.SizeMemoryCache, 0.18)

	// Instantiate 4
	msg4 := []byte(`{"verifier": "fred", "beneficiary": "jeff"}`)
	ires, _, err = vm.Instantiate(checksum, env, info, msg4, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// GetMetrics 8
	metrics, err = vm.GetMetrics()
	assert.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(3), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.Equal(t, uint64(0), metrics.SizePinnedMemoryCache)
	require.InEpsilon(t, 5665691, metrics.SizeMemoryCache, 0.18)
}

func TestEventManager(t *testing.T) {
	vm := withVM(t)

	// Create contract
	checksum := createTestContract(t, vm, EVENTS_TEST_CONTRACT)

	deserCost := types.UFraction{1, 1}

	// Instantiate
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Coins{}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg1 := []byte(`{}`)
	ires, _, err := vm.Instantiate(checksum, env, info, msg1, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// Issue Events
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	info = api.MockInfo("alice", nil)
	eventsStr := `[{"type":"ty1","attributes":[{"key":"k11","value":"v11"},{"key":"k12","value":"v12"}]},{"type":"ty2","attributes":[{"key":"k21","value":"v21"},{"key":"k22","value":"v22"}]}]`
	msg2 := []byte(fmt.Sprintf(`{"events":{"events":%s}}`, eventsStr))

	eres1, _, err := vm.Execute(checksum, env, info, msg2, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.Equal(t, 0, len(eres1.Messages))

	var expectedEvents types.Events
	err = expectedEvents.UnmarshalJSON([]byte(eventsStr))
	require.NoError(t, err)

	require.Equal(t, []types.Event(expectedEvents), eres1.Events)
	require.Equal(t, 0, len(eres1.Attributes))

	// Issue Attributes
	gasMeter3 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	info = api.MockInfo("alice", nil)
	attributesStr := `[{"key":"alice","value":"42"},{"key":"bob","value":"101010"}]`
	msg3 := []byte(fmt.Sprintf(`{"attributes":{"attributes":%s}}`, attributesStr))

	eres2, _, err := vm.Execute(checksum, env, info, msg3, store, *goapi, querier, gasMeter3, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.Equal(t, 0, len(eres2.Messages))

	var expectedAttributes types.EventAttributes
	err = expectedAttributes.UnmarshalJSON([]byte(attributesStr))
	require.NoError(t, err)

	require.Equal(t, 0, len(eres2.Events))
	require.Equal(t, []types.EventAttribute(expectedAttributes), eres2.Attributes)
}

func TestCallCallablePoint(t *testing.T) {
	vm := withVM(t)

	// Create contract
	checksum := createTestContract(t, vm, EVENTS_TEST_CONTRACT)

	deserCost := types.UFraction{1, 1}

	// Instantiate
	gasMeter1 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	// instantiate it with this store
	store := api.NewLookup(gasMeter1)
	goapi := api.NewMockAPI()
	balance := types.Coins{}
	querier := api.DefaultQuerier(api.MOCK_CONTRACT_ADDR, balance)

	env := api.MockEnv()
	info := api.MockInfo("creator", nil)
	msg1 := []byte(`{}`)
	ires, _, err := vm.Instantiate(checksum, env, info, msg1, store, *goapi, querier, gasMeter1, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.Equal(t, 0, len(ires.Messages))

	// Issue Events
	gasMeter2 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	name := "add_events_dyn"
	nameBin, err := json.Marshal(name)
	require.NoError(t, err)
	eventsIn := types.Events{
		types.Event{
			Type: "ty1",
			Attributes: types.EventAttributes{
				types.EventAttribute{
					Key:   "alice",
					Value: "101010",
				},
				types.EventAttribute{
					Key:   "bob",
					Value: "42",
				},
			},
		},
		types.Event{
			Type: "ty2",
			Attributes: types.EventAttributes{
				types.EventAttribute{
					Key:   "ALICE",
					Value: "42",
				},
				types.EventAttribute{
					Key:   "BOB",
					Value: "101010",
				},
			},
		},
	}
	eventsInBin, err := eventsIn.MarshalJSON()
	require.NoError(t, err)
	argsEv := [][]byte{eventsInBin}
	argsEvBin, err := json.Marshal(argsEv)
	require.NoError(t, err)
	empty := []types.HumanAddress{}
	emptyBin, err := json.Marshal(empty)
	require.NoError(t, err)

	cres1, events, attributes, _, err := vm.CallCallablePoint(nameBin, checksum, false, emptyBin, env, argsEvBin, store, *goapi, querier, gasMeter2, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.Equal(t, []byte(`null`), cres1)
	require.Equal(t, eventsIn, events)
	require.Equal(t, 0, len(attributes))

	// Issue Attributes
	gasMeter3 := api.NewMockGasMeter(TESTING_GAS_LIMIT)
	store.SetGasMeter(gasMeter2)
	name = "add_attributes_dyn"
	nameBin, err = json.Marshal(name)
	require.NoError(t, err)
	attrsIn := types.EventAttributes{
		types.EventAttribute{
			Key:   "alice",
			Value: "42",
		},
		types.EventAttribute{
			Key:   "bob",
			Value: "101010",
		},
	}
	attrsInBin, err := attrsIn.MarshalJSON()
	require.NoError(t, err)
	argsAt := [][]byte{attrsInBin}
	argsAtBin, err := json.Marshal(argsAt)
	require.NoError(t, err)

	cres2, events, attributes, _, err := vm.CallCallablePoint(nameBin, checksum, false, emptyBin, env, argsAtBin, store, *goapi, querier, gasMeter3, TESTING_GAS_LIMIT, deserCost)
	require.NoError(t, err)
	require.Equal(t, []byte(`null`), cres2)
	require.Equal(t, 0, len(events))
	require.Equal(t, attrsIn, attributes)
}

func TestValidateDynamicLinkInterafce(t *testing.T) {
	vm := withVM(t)

	// Create contract
	checksum := createTestContract(t, vm, EVENTS_TEST_CONTRACT)

	correctInterface := []byte(`[{"name":"add_event_dyn","ty":{"params":["I32","I32","I32"],"results":[]}},{"name":"add_events_dyn","ty":{"params":["I32","I32"],"results":[]}},{"name":"add_attribute_dyn","ty":{"params":["I32","I32","I32"],"results":[]}},{"name":"add_attributes_dyn","ty":{"params":["I32","I32"],"results":[]}}]`)
	res, err := vm.ValidateDynamicLinkInterface(checksum, correctInterface)
	require.NoError(t, err)
	require.Equal(t, []byte(`null`), res)

	wrongInterface := []byte(`[{"name":"add_event","ty":{"params":["I32","I32","I32"],"results":[]}},{"name":"add_events","ty":{"params":["I32","I32"],"results":[]}},{"name":"add_attribute","ty":{"params":["I32","I32","I32"],"results":[]}},{"name":"add_attributes","ty":{"params":["I32","I32"],"results":[]}}]`)
	res, err = vm.ValidateDynamicLinkInterface(checksum, wrongInterface)
	require.NoError(t, err)
	require.Contains(t, string(res), `following functions are not implemented`)
	require.Contains(t, string(res), `add_event`)
	require.Contains(t, string(res), `add_events`)
	require.Contains(t, string(res), `add_attribute`)
	require.Contains(t, string(res), `add_attributes`)
}
