package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	bbr_contract "example/main/beaconblockroot"
	vp_contract "example/main/verifyproof"
	"fmt"
	"math/big"
	"math/rand"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/crypto"
)

type ChainContext struct {
	auth    *bind.TransactOpts
	caller  *bind.CallOpts
	address common.Address
	gAlloc  core.GenesisAlloc
	client  *backends.SimulatedBackend
	bbr_abi *bbr_contract.Beaconblockroot
	vp_abi  *vp_contract.Verifyproof
}

type SimplifiedBeaconBlock map[string]string

var sbb_fields = []string{
	"ParentHash",
	"FeeRecipient",
	"StateRoot",
	"Coinbase",
	"ReceiptHash",
	"Time",
	"TxHash",
	"GasUsed",
	"ProposerSlashings",
	"AttesterSlashings",
	"Deposits",
	"VoluntaryExits",
	"ParentRoot",
	"Slot",
	"Graffiti",
	"PrevRandao",
}

func init_contracts() ChainContext {
	var cc ChainContext

	// Setup a client with authentication
	key, _ := crypto.GenerateKey()
	chainID := big.NewInt(1337) // important for sim-backend
	cc.auth, _ = bind.NewKeyedTransactorWithChainID(key, chainID)
	cc.caller = &bind.CallOpts{Context: context.Background(), Pending: false}

	cc.address = cc.auth.From
	cc.gAlloc = map[common.Address]core.GenesisAccount{
		cc.address: {Balance: big.NewInt(1000000000000000000)},
	}

	// Create the simulated backend
	blockGasLimit := uint64(4712388)
	cc.client = backends.NewSimulatedBackend(cc.gAlloc, blockGasLimit)

	gasPrice, _ := cc.client.SuggestGasPrice(context.Background())
	cc.auth.GasPrice = gasPrice

	// Deploy the two contracts to the chain, grab the bbr contract address for later
	bbr_addr, _, bbr_abi, _ := bbr_contract.DeployBeaconblockroot(cc.auth, cc.client)
	cc.bbr_abi = bbr_abi
	cc.client.Commit()

	_, _, vp_abi, _ := vp_contract.DeployVerifyproof(cc.auth, cc.client)
	cc.vp_abi = vp_abi
	cc.client.Commit()

	// Initialize VerifyProof contract with bbr contract address so it can call through interface
	cc.vp_abi.SetAddressBBR(cc.auth, bbr_addr)
	cc.client.Commit()

	// Print current execution block header as example for visibility
	fmt.Println("")
	fmt.Println("Simulation evm Blockchain running, contracts deployed...")
	fmt.Println("")

	blockNum, _ := cc.client.BlockNumber(context.Background())
	currentNum := new(big.Int).SetUint64(blockNum)
	blockHeader, _ := cc.client.HeaderByNumber(context.Background(), currentNum)
	fmt.Printf("sample EL header: %+v\n", blockHeader)

	return cc
}

// Save simplified, unrolled beacon blocks for verification, and update the BeaconBlockRoot contract
func save_simplified_beacon_block(
	cc ChainContext,
	block_history []SimplifiedBeaconBlock,
	block_timestamps []big.Int,
) ([]SimplifiedBeaconBlock, []big.Int) {
	// The BeaconBlock type has a field called body....
	// which is a BeaconBlockBody type that has a field called execution_payload...
	// which is a ExecutionPayload type which has the eth1 type data
	// and importantly the timestamp which is used as a key in the BeaconBlockRoot contract

	// Therefore with the BeaconBlockRoot we can generate merkle proofs for fields
	// in the CL data: BeaconBlock, BeackonBlockBody and in the EL data: ExecutionPayload if needed

	// With the EL client we can get the ExecutionHeader which has many of the same fields or roots of them...
	// The execution and consensus clients are different
	// Without actually getting at a consensus client, we can simulate some of these fields
	// And flatten out the nested data structures to be a bunch of equal sized byte32 sized strings
	// Simulating it with random data for some interesting LS relevant fields in the CL data
	// (Choosing a number of fields which is a power of 2 for later proof generation convenience)

	blockNum, _ := cc.client.BlockNumber(context.Background())
	//fmt.Printf("blocknum %d\n", blockNum)

	currentNum := new(big.Int).SetUint64(blockNum)
	blockHeader, _ := cc.client.HeaderByNumber(context.Background(), currentNum)

	blockHeaderMap := make(map[string]string)

	r := reflect.ValueOf(blockHeader).Elem()
	rt := r.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		rv := reflect.ValueOf(blockHeader)
		value := reflect.Indirect(rv).FieldByName(field.Name)
		blockHeaderMap[field.Name] = fmt.Sprintf("%+v", value)
		//fmt.Println(field.Name, value.String())
	}

	sbb := make(SimplifiedBeaconBlock)
	for _, field_name := range sbb_fields {
		if val, ok := blockHeaderMap[field_name]; ok {
			// get real values trimmed to size for data if in EL client
			paddedVal := val + strings.Repeat(" ", 32)
			sbb[field_name] = paddedVal[:32]
		} else {
			// generate random placeholder data as standin for CL client
			sbb[field_name] = random_data_value()
		}
	}

	fmt.Println("")
	fmt.Printf("Saving SimplifiedBeaconBlock Data: %+v\n", sbb)

	sbb_root := compute_sbb_root(sbb)
	sbb_timestamp := new(big.Int).SetUint64(blockHeader.Time)

	fmt.Printf("Timestamp %d ----> Computed and set BeaconBlock Root %s\n", sbb_timestamp, sbb_root)

	cc.bbr_abi.Set(cc.auth, sbb_timestamp, sbb_root)
	cc.client.Commit() // finalizes the block and moves to the next

	block_history = append(block_history, sbb)
	block_timestamps = append(block_timestamps, *sbb_timestamp)

	return block_history, block_timestamps
}

func random_data_value() string {
	data := make([]byte, 10)
	for i := range data {
		data[i] = byte(rand.Intn(256))
	}
	hash := sha256.Sum256(data)
	placeholder := hex.EncodeToString(hash[:])
	return placeholder[:32]
}

func test_verifier_true_inputs(cc ChainContext, field_name string,
	sbb SimplifiedBeaconBlock, timestamp big.Int) bool {
	field_proof := generate_proof(field_name, sbb)
	field_value := sbb[field_name]
	field_index := uint64(compute_merkle_index_from_field(field_name))

	//fmt.Printf("BlockData %+v\n", sbb)
	//fmt.Printf("data value %s -- timestamp %d -- proof %+v\n", field_value, timestamp, field_proof)
	//fmt.Println("")
	//fmt.Printf("full merkle tree %+v\n", compute_sbb_merkle_tree(sbb))
	//fmt.Println("")

	accepted, _ := cc.vp_abi.Verify(cc.caller, &timestamp,
		field_value, field_proof, new(big.Int).SetUint64(field_index))
	time.Sleep(1 * time.Second)
	return accepted
}

func test_verifier_wrong_value(cc ChainContext, field_name string,
	sbb SimplifiedBeaconBlock, timestamp big.Int) bool {
	field_proof := generate_proof(field_name, sbb)
	field_value := random_data_value()
	field_index := uint64(compute_merkle_index_from_field(field_name))

	accepted, _ := cc.vp_abi.Verify(cc.caller, &timestamp,
		field_value, field_proof, new(big.Int).SetUint64(field_index))
	time.Sleep(1 * time.Second)
	return accepted
}

func test_verifier_wrong_proof(cc ChainContext, field_name string,
	sbb SimplifiedBeaconBlock, timestamp big.Int) bool {
	field_proof := generate_proof(field_name, sbb)
	// corrupt the proof by simply swapping two values
	field_proof[0], field_proof[1] = field_proof[1], field_proof[0]
	field_value := random_data_value()
	field_index := uint64(compute_merkle_index_from_field(field_name))

	accepted, _ := cc.vp_abi.Verify(cc.caller, &timestamp,
		field_value, field_proof, new(big.Int).SetUint64(field_index))
	time.Sleep(1 * time.Second)
	return accepted
}

func main() {
	fmt.Println("")
	fmt.Println("Initializing simulation evm Blockchain with BeaconBlockRoot and Verifier contracts...")

	cc := init_contracts()

	fmt.Println("\n")
	time.Sleep(7 * time.Second)

	fmt.Println("Saving a few blocks, setting computed roots in onchain BeaconBlockRoot contract...")

	block_history := []SimplifiedBeaconBlock{}
	block_timestamps := []big.Int{}

	for i := 0; i < 5; i++ {
		block_history, block_timestamps = save_simplified_beacon_block(cc, block_history, block_timestamps)
		time.Sleep(4 * time.Second) // So that the timestamps used as keys in contract are different
	}
	fmt.Println("\n\n")
	time.Sleep(10 * time.Second)

	fmt.Println("Running verification tests...")
	fmt.Println("")

	// Verify EL data like the coinbase from the 2nd recorded block
	fmt.Println("Verify EL data like the coinbase from the 2nd saved block given true proof...")
	result := test_verifier_true_inputs(cc, "Coinbase", block_history[1], block_timestamps[1])
	fmt.Printf("Expected verification: true \nActual verification: %+v\n\n", result)

	// Verify EL data like parenthash from the 4th block
	fmt.Println("Verify EL data like parenthash from the 4th saved block given true proof...")
	result = test_verifier_true_inputs(cc, "ParentHash", block_history[3], block_timestamps[3])
	fmt.Printf("Expected verification: true \nActual verification: %+v\n\n", result)

	// Verify CL data like the slashed validators from the 3rd block
	fmt.Println("Verify CL data like proposer slashings from the 3rd block given true proof...")
	result = test_verifier_true_inputs(cc, "ProposerSlashings", block_history[2], block_timestamps[2])
	fmt.Printf("Expected verification: true \nActual verification: %+v\n\n", result)

	// Verifier should fail when given the wrong data (but the right proof)
	fmt.Println("Verifier should fail when given the wrong data (but a correct proof)...")
	result = test_verifier_wrong_value(cc, "ParentRoot", block_history[1], block_timestamps[1])
	fmt.Printf("Expected verification: false \nActual verification: %+v\n\n", result)

	// Verify should fail when given the right data but a corrupted proof
	fmt.Println("Verifier should fail when given the right data but a corrupted proof...")
	result = test_verifier_wrong_proof(cc, "Deposits", block_history[4], block_timestamps[4])
	fmt.Printf("Expected verification: false \nActual verification: %+v\n\n", result)

	// Verifier should fail when given the wrong timestamp as a key to BeaconBlockRoot contract
	fmt.Println("Verifier should fail with wrong timestamp as a key to BeaconBlockRoot contract...")
	result = test_verifier_true_inputs(cc, "ProposerSlashings", block_history[2], block_timestamps[0])
	fmt.Printf("Expected verification: false \nActual verification: %+v\n\n", result)

}
