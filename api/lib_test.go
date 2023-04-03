package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/line/wasmvm/types"
)

const (
	TESTING_FEATURES     = "staking,stargate,iterator"
	TESTING_PRINT_DEBUG  = false
	TESTING_GAS_LIMIT    = uint64(500_000_000_000) // ~0.5ms
	TESTING_MEMORY_LIMIT = 32                      // MiB
	TESTING_CACHE_SIZE   = 100                     // MiB
)

func TestInitAndReleaseCache(t *testing.T) {
	dataDir := "/foo"
	_, err := InitCache(dataDir, TESTING_FEATURES, TESTING_CACHE_SIZE, TESTING_MEMORY_LIMIT)
	require.Error(t, err)

	tmpdir, err := ioutil.TempDir("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	cache, err := InitCache(tmpdir, TESTING_FEATURES, TESTING_CACHE_SIZE, TESTING_MEMORY_LIMIT)
	require.NoError(t, err)
	ReleaseCache(cache)
}

func TestInitCacheEmptyFeatures(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "wasmvm-testing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)
	cache, err := InitCache(tmpdir, "", TESTING_CACHE_SIZE, TESTING_MEMORY_LIMIT)
	ReleaseCache(cache)
}

func withCache(t *testing.T) (Cache, func()) {
	tmpdir, err := ioutil.TempDir("", "wasmvm-testing")
	require.NoError(t, err)
	cache, err := InitCache(tmpdir, TESTING_FEATURES, TESTING_CACHE_SIZE, TESTING_MEMORY_LIMIT)
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tmpdir)
		ReleaseCache(cache)
	}
	return cache, cleanup
}

func TestCreateAndGet(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := Create(cache, wasm)
	require.NoError(t, err)

	code, err := GetCode(cache, checksum)
	require.NoError(t, err)
	require.Equal(t, wasm, code)
}

func TestCreateFailsWithBadData(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm := []byte("some invalid data")
	_, err := Create(cache, wasm)
	require.Error(t, err)
}

func TestPin(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := Create(cache, wasm)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	// Can be called again with no effect
	err = Pin(cache, checksum)
	require.NoError(t, err)
}

func TestPinErrors(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	var err error

	// Nil checksum (errors in wasmvm Rust code)
	var nilChecksum []byte
	err = Pin(cache, nilChecksum)
	require.ErrorContains(t, err, "Null/Nil argument: checksum")

	// Checksum too short (errors in wasmvm Rust code)
	brokenChecksum := []byte{0x3f, 0xd7, 0x5a, 0x76}
	err = Pin(cache, brokenChecksum)
	require.ErrorContains(t, err, "Checksum not of length 32")

	// Unknown checksum (errors in cosmwasm-vm)
	unknownChecksum := []byte{
		0x72, 0x2c, 0x8c, 0x99, 0x3f, 0xd7, 0x5a, 0x76, 0x27, 0xd6, 0x9e, 0xd9, 0x41, 0x34,
		0x4f, 0xe2, 0xa1, 0x42, 0x3a, 0x3e, 0x75, 0xef, 0xd3, 0xe6, 0x77, 0x8a, 0x14, 0x28,
		0x84, 0x22, 0x71, 0x04,
	}
	err = Pin(cache, unknownChecksum)
	require.ErrorContains(t, err, "No such file or directory")
}

func TestUnpin(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
	require.NoError(t, err)

	checksum, err := Create(cache, wasm)
	require.NoError(t, err)

	err = Pin(cache, checksum)
	require.NoError(t, err)

	err = Unpin(cache, checksum)
	require.NoError(t, err)

	// Can be called again with no effect
	err = Unpin(cache, checksum)
	require.NoError(t, err)
}

func TestUnpinErrors(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	var err error

	// Nil checksum (errors in wasmvm Rust code)
	var nilChecksum []byte
	err = Unpin(cache, nilChecksum)
	require.ErrorContains(t, err, "Null/Nil argument: checksum")

	// Checksum too short (errors in wasmvm Rust code)
	brokenChecksum := []byte{0x3f, 0xd7, 0x5a, 0x76}
	err = Unpin(cache, brokenChecksum)
	require.ErrorContains(t, err, "Checksum not of length 32")

	// No error case triggered in cosmwasm-vm is known right now
}

func TestGetMetrics(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// GetMetrics 1
	metrics, err := GetMetrics(cache)
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{}, metrics)

	// Create contract
	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := Create(cache, wasm)
	require.NoError(t, err)

	// GetMetrics 2
	metrics, err = GetMetrics(cache)
	require.NoError(t, err)
	assert.Equal(t, &types.Metrics{}, metrics)

	// Instantiate 1
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Coins{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg1 := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, _, _, err = Instantiate(cache, checksum, env, info, msg1, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 3
	metrics, err = GetMetrics(cache)
	assert.NoError(t, err)
	require.Equal(t, uint32(0), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 5665691, metrics.SizeMemoryCache, 0.18)

	// Instantiate 2
	msg2 := []byte(`{"verifier": "fred", "beneficiary": "susi"}`)
	_, _, _, _, err = Instantiate(cache, checksum, env, info, msg2, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 4
	metrics, err = GetMetrics(cache)
	assert.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 5665691, metrics.SizeMemoryCache, 0.18)

	// Pin
	err = Pin(cache, checksum)
	require.NoError(t, err)

	// GetMetrics 5
	metrics, err = GetMetrics(cache)
	assert.NoError(t, err)
	require.Equal(t, uint32(2), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 5665691, metrics.SizePinnedMemoryCache, 0.18)
	require.InEpsilon(t, 5665691, metrics.SizeMemoryCache, 0.18)

	// Instantiate 3
	msg3 := []byte(`{"verifier": "fred", "beneficiary": "bert"}`)
	_, _, _, _, err = Instantiate(cache, checksum, env, info, msg3, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 6
	metrics, err = GetMetrics(cache)
	assert.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(2), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(1), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.InEpsilon(t, 5665691, metrics.SizePinnedMemoryCache, 0.18)
	require.InEpsilon(t, 5665691, metrics.SizeMemoryCache, 0.18)

	// Unpin
	err = Unpin(cache, checksum)
	require.NoError(t, err)

	// GetMetrics 7
	metrics, err = GetMetrics(cache)
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
	_, _, _, _, err = Instantiate(cache, checksum, env, info, msg4, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// GetMetrics 8
	metrics, err = GetMetrics(cache)
	assert.NoError(t, err)
	require.Equal(t, uint32(1), metrics.HitsPinnedMemoryCache)
	require.Equal(t, uint32(3), metrics.HitsMemoryCache)
	require.Equal(t, uint32(1), metrics.HitsFsCache)
	require.Equal(t, uint64(0), metrics.ElementsPinnedMemoryCache)
	require.Equal(t, uint64(1), metrics.ElementsMemoryCache)
	require.Equal(t, uint64(0), metrics.SizePinnedMemoryCache)
	require.InEpsilon(t, 5665691, metrics.SizeMemoryCache, 0.18)
}

func TestInstantiate(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()

	// create contract
	wasm, err := ioutil.ReadFile("./testdata/hackatom.wasm")
	require.NoError(t, err)
	checksum, err := Create(cache, wasm)
	require.NoError(t, err)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := GasMeter(gasMeter)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Coins{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, eventsData, attributesData, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x12e722d7c), cost)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Equal(t, 0, len(result.Ok.Messages))

	// make sure it does not uses EventManager
	var events types.Events
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	var attributes types.EventAttributes
	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))
}

func TestExecute(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createTestContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Coins{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	start := time.Now()
	res, eventsData, attributesData, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff := time.Now().Sub(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x12e722d7c), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)

	// make sure it does not uses EventManager
	var events types.Events
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	var attributes types.EventAttributes
	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

	// execute with the same store
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	info = MockInfoBin(t, "fred")
	start = time.Now()
	res, eventsData, attributesData, cost, err = Execute(cache, checksum, env, info, []byte(`{"release":{}}`), &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff = time.Now().Sub(start)
	require.NoError(t, err)
	assert.Equal(t, uint64(0x21f7a66d0), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)

	// make sure it does not uses EventManager
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

	// make sure it read the balance properly and we got 250 atoms
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Equal(t, 1, len(result.Ok.Messages))

	// Ensure we got our custom event
	assert.Equal(t, len(result.Ok.Events), 1)
	ev := result.Ok.Events[0]
	assert.Equal(t, ev.Type, "hackatom")
	assert.Equal(t, len(ev.Attributes), 1)
	assert.Equal(t, ev.Attributes[0].Key, "action")
	assert.Equal(t, ev.Attributes[0].Value, "release")

	dispatch := result.Ok.Messages[0].Msg
	require.NotNil(t, dispatch.Bank, "%#v", dispatch)
	require.NotNil(t, dispatch.Bank.Send, "%#v", dispatch)
	send := dispatch.Bank.Send
	assert.Equal(t, "bob", send.ToAddress)
	assert.Equal(t, balance, send.Amount)
	// check the data is properly formatted
	expectedData := []byte{0xF0, 0x0B, 0xAA}
	assert.Equal(t, expectedData, result.Ok.Data)
}

func TestExecuteCpuLoop(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createTestContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Coins{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	start := time.Now()
	res, eventsData, attributesData, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff := time.Now().Sub(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x12e722d7c), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)

	// make sure it does not uses EventManager
	var events types.Events
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	var attributes types.EventAttributes
	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

	// execute a cpu loop
	maxGas := uint64(40_000_000)
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	start = time.Now()
	res, _, _, cost, err = Execute(cache, checksum, env, info, []byte(`{"cpu_loop":{}}`), &igasMeter2, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	diff = time.Now().Sub(start)
	require.Error(t, err)
	assert.Equal(t, cost, maxGas)
	t.Logf("CPULoop Time (%d gas): %s\n", cost, diff)
}

func TestExecuteStorageLoop(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createTestContract(t, cache)

	maxGas := TESTING_GAS_LIMIT
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Coins{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, eventsData, attributesData, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// make sure it does not uses EventManager
	var events types.Events
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	var attributes types.EventAttributes
	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

	// execute a storage loop
	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	start := time.Now()
	res, _, _, cost, err = Execute(cache, checksum, env, info, []byte(`{"storage_loop":{}}`), &igasMeter2, store, api, &querier, maxGas, TESTING_PRINT_DEBUG)
	diff := time.Now().Sub(start)
	require.Error(t, err)
	t.Logf("StorageLoop Time (%d gas): %s\n", cost, diff)
	t.Logf("Gas used: %d\n", gasMeter2.GasConsumed())
	t.Logf("Wasm gas: %d\n", cost)

	// the "sdk gas" * GasMultiplier + the wasm cost should equal the maxGas (or be very close)
	totalCost := cost + gasMeter2.GasConsumed()
	require.Equal(t, int64(maxGas), int64(totalCost))
}

func TestExecuteUserErrorsInApiCalls(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createTestContract(t, cache)

	maxGas := TESTING_GAS_LIMIT
	gasMeter1 := NewMockGasMeter(maxGas)
	igasMeter1 := GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	balance := types.Coins{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	defaultApi := NewMockAPI()
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, eventsData, attributesData, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, defaultApi, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	var events types.Events
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	var attributes types.EventAttributes
	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	gasMeter2 := NewMockGasMeter(maxGas)
	igasMeter2 := GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "fred")
	failingApi := NewMockFailureAPI()
	res, eventsData, attributesData, _, err = Execute(cache, checksum, env, info, []byte(`{"user_errors_in_api_calls":{}}`), &igasMeter2, store, failingApi, &querier, maxGas, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

}

func TestMigrate(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createTestContract(t, cache)

	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := GasMeter(gasMeter)
	// instantiate it with this store
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	balance := types.Coins{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)

	res, eventsData, attributesData, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// make sure it does not uses EventManager
	var events types.Events
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	var attributes types.EventAttributes
	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	// verifier is fred
	query := []byte(`{"verifier":{}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qres types.QueryResponse
	err = json.Unmarshal(data, &qres)
	require.NoError(t, err)
	require.Equal(t, "", qres.Err)
	require.Equal(t, string(qres.Ok), `{"verifier":"fred"}`)

	// migrate to a new verifier - alice
	// we use the same code blob as we are testing hackatom self-migration
	res, eventsData, attributesData, _, err = Migrate(cache, checksum, env, []byte(`{"verifier":"alice"}`), &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// make sure it does not uses EventManager
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

	// should update verifier to alice
	data, _, err = Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qres2 types.QueryResponse
	err = json.Unmarshal(data, &qres2)
	require.NoError(t, err)
	require.Equal(t, "", qres2.Err)
	require.Equal(t, `{"verifier":"alice"}`, string(qres2.Ok))
}

func TestMultipleInstances(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createTestContract(t, cache)

	// instance1 controlled by fred
	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := GasMeter(gasMeter1)
	store1 := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Coins{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "regen")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, eventsData, attributesData, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store1, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	// we now count wasm gas charges and db writes
	assert.Equal(t, uint64(0x12c4f266c), cost)

	// make sure it does not uses EventManager
	var eventsByEventManager types.Events
	err = eventsByEventManager.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(eventsByEventManager))

	var attributesByEventManager types.EventAttributes
	err = attributesByEventManager.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributesByEventManager))

	// instance2 controlled by mary
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := GasMeter(gasMeter2)
	store2 := NewLookup(gasMeter2)
	info = MockInfoBin(t, "chrous")
	msg = []byte(`{"verifier": "mary", "beneficiary": "sue"}`)
	res, eventsData, attributesData, cost, err = Instantiate(cache, checksum, env, info, msg, &igasMeter2, store2, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x12d8b01cc), cost)

	// make sure it does not uses EventManager
	err = eventsByEventManager.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(eventsByEventManager))

	err = attributesByEventManager.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributesByEventManager))

	// fail to execute store1 with mary
	resp := exec(t, cache, checksum, "mary", store1, api, querier, 0x115070970)
	require.Equal(t, "Unauthorized", resp.Err)

	// succeed to execute store1 with fred
	resp = exec(t, cache, checksum, "fred", store1, api, querier, 0x21e7a0dd0)
	require.Equal(t, "", resp.Err)
	require.Equal(t, 1, len(resp.Ok.Messages))
	attributes := resp.Ok.Attributes
	require.Equal(t, 2, len(attributes))
	require.Equal(t, "destination", attributes[1].Key)
	require.Equal(t, "bob", attributes[1].Value)

	// succeed to execute store2 with mary
	resp = exec(t, cache, checksum, "mary", store2, api, querier, 0x21efa3a50)
	require.Equal(t, "", resp.Err)
	require.Equal(t, 1, len(resp.Ok.Messages))
	attributes = resp.Ok.Attributes
	require.Equal(t, 2, len(attributes))
	require.Equal(t, "destination", attributes[1].Key)
	require.Equal(t, "sue", attributes[1].Value)
}

func TestSudo(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createTestContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Coins{types.NewCoin(250, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	res, eventsData, attributesData, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// make sure it does not uses EventManager
	var events types.Events
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	var attributes types.EventAttributes
	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

	// call sudo with same store
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	msg = []byte(`{"steal_funds":{"recipient":"community-pool","amount":[{"amount":"700","denom":"gold"}]}}`)
	res, eventsData, attributesData, _, err = Sudo(cache, checksum, env, msg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// make sure it does not uses EventManager
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

	// make sure it blindly followed orders
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Equal(t, 1, len(result.Ok.Messages))
	dispatch := result.Ok.Messages[0].Msg
	require.NotNil(t, dispatch.Bank, "%#v", dispatch)
	require.NotNil(t, dispatch.Bank.Send, "%#v", dispatch)
	send := dispatch.Bank.Send
	assert.Equal(t, "community-pool", send.ToAddress)
	expectedPayout := types.Coins{types.NewCoin(700, "gold")}
	assert.Equal(t, expectedPayout, send.Amount)
}

func TestDispatchSubmessage(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createReflectContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)
	res, eventsData, attributesData, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// make sure it does not uses EventManager
	var events types.Events
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	var attributes types.EventAttributes
	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

	// dispatch a submessage
	var id uint64 = 1234
	payload := types.SubMsg{
		ID: id,
		Msg: types.CosmosMsg{Bank: &types.BankMsg{Send: &types.SendMsg{
			ToAddress: "friend",
			Amount:    types.Coins{types.NewCoin(1, "token")},
		}}},
		ReplyOn: types.ReplyAlways,
	}
	payloadBin, err := json.Marshal(payload)
	require.NoError(t, err)
	payloadMsg := []byte(fmt.Sprintf(`{"reflect_sub_msg":{"msgs":[%s]}}`, string(payloadBin)))

	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	res, eventsData, attributesData, _, err = Execute(cache, checksum, env, info, payloadMsg, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// make sure it does not uses EventManager
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

	// make sure it blindly followed orders
	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Equal(t, 1, len(result.Ok.Messages))
	dispatch := result.Ok.Messages[0]
	assert.Equal(t, id, dispatch.ID)
	assert.Equal(t, payload.Msg, dispatch.Msg)
	assert.Nil(t, dispatch.GasLimit)
	assert.Equal(t, payload.ReplyOn, dispatch.ReplyOn)
}

func TestReplyAndQuery(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createReflectContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, nil)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")

	msg := []byte(`{}`)
	res, eventsData, attributesData, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// make sure it does not uses EventManager
	var eventsByEventManager types.Events
	err = eventsByEventManager.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(eventsByEventManager))

	var attributesByEventManager types.EventAttributes
	err = attributesByEventManager.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributesByEventManager))

	var id uint64 = 1234
	data := []byte("foobar")
	events := types.Events{{
		Type: "message",
		Attributes: types.EventAttributes{{
			Key:   "signer",
			Value: "caller-addr",
		}},
	}}
	reply := types.Reply{
		ID: id,
		Result: types.SubcallResult{
			Ok: &types.SubcallResponse{
				Events: events,
				Data:   data,
			},
		},
	}
	replyBin, err := json.Marshal(reply)
	require.NoError(t, err)

	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	env = MockEnvBin(t)
	res, eventsData, attributesData, _, err = Reply(cache, checksum, env, replyBin, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)

	// make sure it does not uses EventManager
	err = eventsByEventManager.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(eventsByEventManager))

	err = attributesByEventManager.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributesByEventManager))

	// now query the state to see if it stored the data properly
	badQuery := []byte(`{"sub_msg_result":{"id":7777}}`)
	res, _, err = Query(cache, checksum, env, badQuery, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	requireQueryError(t, res)

	query := []byte(`{"sub_msg_result":{"id":1234}}`)
	res, _, err = Query(cache, checksum, env, query, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	qres := requireQueryOk(t, res)

	var stored types.Reply
	err = json.Unmarshal(qres, &stored)
	require.NoError(t, err)
	assert.Equal(t, id, stored.ID)
	require.NotNil(t, stored.Result.Ok)
	val := stored.Result.Ok
	require.Equal(t, data, val.Data)
	require.Equal(t, events, val.Events)
}

func requireOkResponse(t *testing.T, res []byte, expectedMsgs int) {
	var result types.ContractResult
	err := json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Equal(t, "", result.Err)
	require.Equal(t, expectedMsgs, len(result.Ok.Messages))
}

func requireQueryError(t *testing.T, res []byte) {
	var result types.QueryResponse
	err := json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Empty(t, result.Ok)
	require.NotEmpty(t, result.Err)
}

func requireQueryOk(t *testing.T, res []byte) []byte {
	var result types.QueryResponse
	err := json.Unmarshal(res, &result)
	require.NoError(t, err)
	require.Empty(t, result.Err)
	require.NotEmpty(t, result.Ok)
	return result.Ok
}

func createTestContract(t *testing.T, cache Cache) []byte {
	return createContract(t, cache, "./testdata/hackatom.wasm")
}

func createQueueContract(t *testing.T, cache Cache) []byte {
	return createContract(t, cache, "./testdata/queue.wasm")
}

func createReflectContract(t *testing.T, cache Cache) []byte {
	return createContract(t, cache, "./testdata/reflect.wasm")
}

func createEventsContract(t *testing.T, cache Cache) []byte {
	return createContract(t, cache, "./testdata/events.wasm")
}

func createNumberContract(t *testing.T, cache Cache) []byte {
	return createContract(t, cache, "./testdata/number.wasm")
}

func createIntermediateNumberContract(t *testing.T, cache Cache) []byte {
	return createContract(t, cache, "./testdata/intermediate_number.wasm")
}

func createCallNumberContract(t *testing.T, cache Cache) []byte {
	return createContract(t, cache, "./testdata/call_number.wasm")
}

func createContract(t *testing.T, cache Cache, wasmFile string) []byte {
	wasm, err := ioutil.ReadFile(wasmFile)
	require.NoError(t, err)
	checksum, err := Create(cache, wasm)
	require.NoError(t, err)
	return checksum
}

// exec runs the handle tx with the given signer
func exec(t *testing.T, cache Cache, checksum []byte, signer types.HumanAddress, store KVStore, api *GoAPI, querier Querier, gasExpected uint64) types.ContractResult {
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := GasMeter(gasMeter)
	env := MockEnvBin(t)
	info := MockInfoBin(t, signer)
	res, _, _, cost, err := Execute(cache, checksum, env, info, []byte(`{"release":{}}`), &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	assert.Equal(t, gasExpected, cost)

	var result types.ContractResult
	err = json.Unmarshal(res, &result)
	require.NoError(t, err)
	return result
}

func TestQuery(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createTestContract(t, cache)

	// set up contract
	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := GasMeter(gasMeter1)
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, types.Coins{types.NewCoin(100, "ATOM")})
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{"verifier": "fred", "beneficiary": "bob"}`)
	_, _, _, _, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)

	// invalid query
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	query := []byte(`{"Raw":{"val":"config"}}`)
	data, _, err := Query(cache, checksum, env, query, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var badResp types.QueryResponse
	err = json.Unmarshal(data, &badResp)
	require.NoError(t, err)
	require.Contains(t, badResp.Err, "Error parsing into type hackatom::msg::QueryMsg: unknown variant `Raw`, expected one of")

	// make a valid query
	gasMeter3 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter3 := GasMeter(gasMeter3)
	store.SetGasMeter(gasMeter3)
	query = []byte(`{"verifier":{}}`)
	data, _, err = Query(cache, checksum, env, query, &igasMeter3, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qres types.QueryResponse
	err = json.Unmarshal(data, &qres)
	require.NoError(t, err)
	require.Equal(t, "", qres.Err)
	require.Equal(t, string(qres.Ok), `{"verifier":"fred"}`)
}

func TestHackatomQuerier(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createTestContract(t, cache)

	// set up contract
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	initBalance := types.Coins{types.NewCoin(1234, "ATOM"), types.NewCoin(65432, "ETH")}
	querier := DefaultQuerier("foobar", initBalance)

	// make a valid query to the other address
	query := []byte(`{"other_balance":{"address":"foobar"}}`)
	// TODO The query happens before the contract is initialized. How is this legal?
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qres types.QueryResponse
	err = json.Unmarshal(data, &qres)
	require.NoError(t, err)
	require.Equal(t, "", qres.Err)
	var balances types.AllBalancesResponse
	err = json.Unmarshal(qres.Ok, &balances)
	require.Equal(t, balances.Amount, initBalance)
}

func TestCustomReflectQuerier(t *testing.T) {
	type CapitalizedQuery struct {
		Text string `json:"text"`
	}

	type QueryMsg struct {
		Capitalized *CapitalizedQuery `json:"capitalized,omitempty"`
		// There are more queries but we don't use them yet
		// https://github.com/line/cosmwasm/blob/v0.14.0-0.4.0/contracts/reflect/src/msg.rs#L38-L57
	}

	type CapitalizedResponse struct {
		Text string `json:"text"`
	}

	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createReflectContract(t, cache)

	// set up contract
	gasMeter := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter := GasMeter(gasMeter)
	store := NewLookup(gasMeter)
	api := NewMockAPI()
	initBalance := types.Coins{types.NewCoin(1234, "ATOM")}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, initBalance)
	// we need this to handle the custom requests from the reflect contract
	innerQuerier := querier.(MockQuerier)
	innerQuerier.Custom = ReflectCustom{}
	querier = Querier(innerQuerier)

	// make a valid query to the other address
	queryMsg := QueryMsg{
		Capitalized: &CapitalizedQuery{
			Text: "small Frys :)",
		},
	}
	query, err := json.Marshal(queryMsg)
	require.NoError(t, err)
	env := MockEnvBin(t)
	data, _, err := Query(cache, checksum, env, query, &igasMeter, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	require.NoError(t, err)
	var qres types.QueryResponse
	err = json.Unmarshal(data, &qres)
	require.NoError(t, err)
	require.Equal(t, "", qres.Err)

	var response CapitalizedResponse
	err = json.Unmarshal(qres.Ok, &response)
	require.NoError(t, err)
	require.Equal(t, "SMALL FRYS :)", response.Text)
}

func TestEventManager(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createEventsContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Coins{}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{}`)

	start := time.Now()
	res, eventsData, attributesData, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff := time.Now().Sub(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xc5c6f370), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)

	// make sure it does not uses EventManager
	var events types.Events
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	var attributes types.EventAttributes
	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

	// issue events with EventManager
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := GasMeter(gasMeter2)
	store.SetGasMeter(gasMeter2)
	info = MockInfoBin(t, "alice")
	eventsStr := `[{"type":"ty1","attributes":[{"key":"k11","value":"v11"},{"key":"k12","value":"v12"}]},{"type":"ty2","attributes":[{"key":"k21","value":"v21"},{"key":"k22","value":"v22"}]}]`
	msg2 := []byte(fmt.Sprintf(`{"events":{"events":%s}}`, eventsStr))

	start = time.Now()
	res, eventsData, attributesData, cost, err = Execute(cache, checksum, env, info, msg2, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff = time.Now().Sub(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x1d0d83e80), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)

	// check events and attributes
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)

	var expectedEvents types.Events
	err = expectedEvents.UnmarshalJSON([]byte(eventsStr))
	require.NoError(t, err)

	require.Equal(t, expectedEvents, events)

	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

	// issue attributes with EventManager
	gasMeter3 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter3 := GasMeter(gasMeter3)
	store.SetGasMeter(gasMeter3)
	info = MockInfoBin(t, "alice")
	attributesStr := `[{"key":"alice","value":"42"},{"key":"bob","value":"101010"}]`
	msg3 := []byte(fmt.Sprintf(`{"attributes":{"attributes":%s}}`, attributesStr))

	start = time.Now()
	res, eventsData, attributesData, cost, err = Execute(cache, checksum, env, info, msg3, &igasMeter3, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff = time.Now().Sub(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x13d2bd4d0), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)

	// check events and attributes
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)

	var expectedAttributes types.EventAttributes
	err = expectedAttributes.UnmarshalJSON([]byte(attributesStr))
	require.NoError(t, err)
	require.Equal(t, expectedAttributes, attributes)
}

// This is used for TestDynamicReadWritePermission
type MockQuerier_read_write struct {
	Bank    BankQuerier
	Custom  CustomQuerier
	usedGas uint64
}

var _ types.Querier = MockQuerier_read_write{}

func (q MockQuerier_read_write) GasConsumed() uint64 {
	return q.usedGas
}

func DefaultQuerier_read_write(contractAddr string, coins types.Coins) Querier {
	balances := map[string]types.Coins{
		contractAddr: coins,
	}
	return MockQuerier_read_write{
		Bank:    NewBankQuerier(balances),
		Custom:  NoCustom{},
		usedGas: 0,
	}
}

func (q MockQuerier_read_write) Query(request types.QueryRequest, _gasLimit uint64) ([]byte, error) {
	marshaled, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	q.usedGas += uint64(len(marshaled))
	if request.Bank != nil {
		return q.Bank.Query(request.Bank)
	}
	if request.Custom != nil {
		return q.Custom.Query(request.Custom)
	}
	if request.Staking != nil {
		return nil, types.UnsupportedRequest{"staking"}
	}
	if request.Wasm != nil {
		// This value is set for use with TestDynamicReadWritePermission.
		// 42 is meaningless.
		return []byte(`{"value":42}`), nil
	}
	return nil, types.Unknown{}
}

func TestDynamicReadWritePermission(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum_number := createNumberContract(t, cache)
	checksum_intermediate_number := createIntermediateNumberContract(t, cache)
	checksum_call_number := createCallNumberContract(t, cache)

	// init callee
	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := GasMeter(gasMeter1)
	calleeStore := NewLookup(gasMeter1)
	calleeEnv := MockEnv()
	calleeEnv.Contract.Address = "number_addr"
	calleeEnvBin, err := json.Marshal(calleeEnv)
	require.NoError(t, err)

	// init intermediate
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := GasMeter(gasMeter2)
	intermediateStore := NewLookup(gasMeter2)
	intermediateEnv := MockEnv()
	intermediateEnv.Contract.Address = "intermediate_number_addr"
	intermediateEnvBin, err := json.Marshal(intermediateEnv)
	require.NoError(t, err)

	// init caller
	gasMeter3 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter3 := GasMeter(gasMeter3)
	callerStore := NewLookup(gasMeter3)
	callerEnv := MockEnv()
	callerEnv.Contract.Address = "call_number_address"
	callerEnvBin, err := json.Marshal(callerEnv)
	require.NoError(t, err)

	// prepare querier
	balance := types.Coins{}
	info := MockInfoBin(t, "someone")
	querier := DefaultQuerier_read_write(calleeEnv.Contract.Address, balance)

	// make api mock with GetContractEnv
	api := NewMockAPI()
	mockGetContractEnv := func(addr string, inputSize uint64) (Env, *Cache, KVStore, Querier, GasMeter, []byte, uint64, uint64, error) {
		if addr == calleeEnv.Contract.Address {
			return calleeEnv, &cache, calleeStore, querier, GasMeter(NewMockGasMeter(TESTING_GAS_LIMIT)), checksum_number, 0, 0, nil
		} else if addr == intermediateEnv.Contract.Address {
			return intermediateEnv, &cache, intermediateStore, querier, GasMeter(NewMockGasMeter(TESTING_GAS_LIMIT)), checksum_intermediate_number, 0, 0, nil
		} else {
			return Env{}, nil, nil, nil, nil, []byte{}, 0, 0, fmt.Errorf("unexpected address")
		}
	}
	api.GetContractEnv = mockGetContractEnv

	// instantiate number contract
	start := time.Now()
	msg := []byte(`{"value":42}`)
	res, _, _, cost, err := Instantiate(cache, checksum_number, calleeEnvBin, info, msg, &igasMeter1, calleeStore, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff := time.Now().Sub(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xd50318f0), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)

	// instantiate intermediate_number contract
	start = time.Now()
	msg = []byte(`{"callee_addr":"number_addr"}`)
	res, _, _, cost, err = Instantiate(cache, checksum_intermediate_number, intermediateEnvBin, info, msg, &igasMeter2, intermediateStore, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff = time.Now().Sub(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xeb087500), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)

	// instantiate call_number contract
	start = time.Now()
	msg = []byte(`{"callee_addr":"intermediate_number_addr"}`)
	res, _, _, cost, err = Instantiate(cache, checksum_call_number, callerEnvBin, info, msg, &igasMeter3, callerStore, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff = time.Now().Sub(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xedd72560), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)

	// fail to execute when calling `add`
	// The intermediate_number contract is intentionally designed so that the `add` function has read-only permission.
	// The following test fails because of inheritance from read-only permission to read-write permission.
	gasMeter4 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter4 := GasMeter(gasMeter4)
	intermediateStore.SetGasMeter(gasMeter4)
	msg4 := []byte(`{"add":{"value":5}}`)

	start = time.Now()
	_, _, _, cost, err = Execute(cache, checksum_call_number, callerEnvBin, info, msg4, &igasMeter4, callerStore, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff = time.Now().Sub(start)
	require.ErrorContains(t, err, "It is not possible to inherit from read-only permission to read-write permission")
	assert.Equal(t, uint64(0x19369c5f0), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)

	// succeed to execute when calling `sub`
	// The intermediate_number contract is designed so that the `sub` function has read-write permission.
	// The following test succeeds because the permissions are properly inherited.
	gasMeter5 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter5 := GasMeter(gasMeter5)
	intermediateStore.SetGasMeter(gasMeter5)
	msg5 := []byte(`{"sub":{"value":5}}`)

	start = time.Now()
	_, _, _, cost, err = Execute(cache, checksum_call_number, callerEnvBin, info, msg5, &igasMeter5, callerStore, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff = time.Now().Sub(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0x535da85b0), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)
}

func TestCallCallablePoint(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createEventsContract(t, cache)

	gasMeter1 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter1 := GasMeter(gasMeter1)
	// instantiate it with this store
	store := NewLookup(gasMeter1)
	api := NewMockAPI()
	balance := types.Coins{}
	querier := DefaultQuerier(MOCK_CONTRACT_ADDR, balance)
	env := MockEnvBin(t)
	info := MockInfoBin(t, "creator")
	msg := []byte(`{}`)

	start := time.Now()
	res, eventsData, attributesData, cost, err := Instantiate(cache, checksum, env, info, msg, &igasMeter1, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff := time.Now().Sub(start)
	require.NoError(t, err)
	requireOkResponse(t, res, 0)
	assert.Equal(t, uint64(0xc5c6f370), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)

	// make sure it does not uses EventManager
	var events types.Events
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	var attributes types.EventAttributes
	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

	// issue events with EventManager
	gasMeter2 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter2 := GasMeter(gasMeter2)
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

	start = time.Now()
	res, eventsData, attributesData, cost, err = CallCallablePoint(nameBin, cache, checksum, false, emptyBin, env, argsEvBin, &igasMeter2, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff = time.Now().Sub(start)
	require.NoError(t, err)
	assert.Equal(t, uint64(0x1766fb680), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)
	require.Equal(t, []byte(`null`), res)

	// check events and attributes
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)

	require.Equal(t, eventsIn, events)

	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)
	require.Equal(t, 0, len(attributes))

	// issue attributes with EventManager
	gasMeter3 := NewMockGasMeter(TESTING_GAS_LIMIT)
	igasMeter3 := GasMeter(gasMeter3)
	store.SetGasMeter(gasMeter3)
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

	start = time.Now()
	res, eventsData, attributesData, cost, err = CallCallablePoint(nameBin, cache, checksum, false, emptyBin, env, argsAtBin, &igasMeter3, store, api, &querier, TESTING_GAS_LIMIT, TESTING_PRINT_DEBUG)
	diff = time.Now().Sub(start)
	require.NoError(t, err)
	assert.Equal(t, uint64(0xd753e6c0), cost)
	t.Logf("Time (%d gas): %s\n", cost, diff)
	require.Equal(t, []byte(`null`), res)

	// check events and attributes
	err = events.UnmarshalJSON(eventsData)
	require.NoError(t, err)
	require.Equal(t, 0, len(events))

	err = attributes.UnmarshalJSON(attributesData)
	require.NoError(t, err)

	require.Equal(t, attrsIn, attributes)
}

func TestValidateDynamicLinkInterafce(t *testing.T) {
	cache, cleanup := withCache(t)
	defer cleanup()
	checksum := createEventsContract(t, cache)

	t.Run("valid interface", func(t *testing.T) {
		correctInterface := []byte(`[{"name":"add_event_dyn","ty":{"params":["I32","I32","I32"],"results":[]}},{"name":"add_events_dyn","ty":{"params":["I32","I32"],"results":[]}},{"name":"add_attribute_dyn","ty":{"params":["I32","I32","I32"],"results":[]}},{"name":"add_attributes_dyn","ty":{"params":["I32","I32"],"results":[]}}]`)
		res, err := ValidateDynamicLinkInterface(cache, checksum, correctInterface)
		require.NoError(t, err)
		assert.Equal(t, []byte(`null`), res)
	})

	t.Run("invalid interface", func(t *testing.T) {
		wrongInterface := []byte(`[{"name":"add_event","ty":{"params":["I32","I32","I32"],"results":[]}},{"name":"add_events","ty":{"params":["I32","I32"],"results":[]}},{"name":"add_attribute","ty":{"params":["I32","I32","I32"],"results":[]}},{"name":"add_attributes","ty":{"params":["I32","I32"],"results":[]}}]`)
		res, err := ValidateDynamicLinkInterface(cache, checksum, wrongInterface)
		require.NoError(t, err)
		assert.Contains(t, string(res), `following functions are not implemented`)
		assert.Contains(t, string(res), `add_event`)
		assert.Contains(t, string(res), `add_events`)
		assert.Contains(t, string(res), `add_attribute`)
		assert.Contains(t, string(res), `add_attributes`)
	})
}
