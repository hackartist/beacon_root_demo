package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Helper functions for generating the merkle proofs for fields in the SimplifiedBeaconBlocks
// Still part of package main so visibility to all types defined there,
// Just split up for organization -- most code in main is about setting up and running the tests
//  agnostic of the underlying algorithmic implementation of the merkle methods

// For simplicity, the number of chunks is a nice power of 2 to avoid edge cases
func compute_sbb_merkle_tree(sbb SimplifiedBeaconBlock) []string {
	node_idx := uint(len(sbb_fields))
	merkle_nodes := make([]string, 2*node_idx)

	for idx, field := range sbb_fields {
		leafData := []byte(sbb[field])
		hash := sha256.Sum256(leafData)
		merkle_nodes[node_idx+uint(idx)] = hex.EncodeToString(hash[:])
		//fmt.Printf("index %d\n", node_idx+uint(idx))
	}

	// reduce until reaching root, ie. when node_idx == 0
	for node_idx > 0 {
		node_idx = get_parent_idx(node_idx)
		for idx := uint(0); idx < node_idx; idx++ {
			offset := node_idx + idx
			combined := []byte(merkle_nodes[2*offset] + merkle_nodes[2*offset+1])
			hash := sha256.Sum256(combined)
			merkle_nodes[offset] = hex.EncodeToString(hash[:])
			//fmt.Printf("node_idx %d -- idx %d -- offset %d\n", node_idx, idx, offset)
		}
	}

	return merkle_nodes
}

func compute_sbb_root(sbb SimplifiedBeaconBlock) string {
	merkle_tree := compute_sbb_merkle_tree(sbb)
	return merkle_tree[1] // indexing starts at 1
}

func compute_merkle_index_from_field(field_name string) uint {
	merkle_idx := uint(len(sbb_fields))
	for _, field := range sbb_fields {
		if field == field_name {
			break
		}
		merkle_idx++
	}
	//fmt.Printf("leaf idx %d for field %s\n", merkle_idx, field_name)

	if merkle_idx == uint(2*len(sbb_fields)) {
		// field_name wasn't found, can't get index or generate proof
		fmt.Printf("field %s wasn't found!\n", field_name)
		return uint(0) // there is no index 0, root is 1, so default to 0 value
	}

	return merkle_idx
}

// instead of computing merkle tree inside this function on ssb argument,
// could be more efficient to compute tree once and pass that in...
func generate_proof(field_name string, sbb SimplifiedBeaconBlock) []string {
	proof := []string{}
	//fmt.Printf("computing proof for field %s\n", field_name)

	merkle_idx := compute_merkle_index_from_field(field_name)
	merkle_tree := compute_sbb_merkle_tree(sbb)
	for merkle_idx > 0 {
		sib_idx := get_sibling_idx(merkle_idx)
		proof = append(proof, merkle_tree[sib_idx])
		merkle_idx = get_parent_idx(merkle_idx)
		//fmt.Printf("sib_idx %d, parent_idx %d\n", sib_idx, merkle_idx)
	}

	return proof
}

func get_sibling_idx(idx uint) uint {
	return idx ^ 1
}

func get_parent_idx(idx uint) uint {
	return idx / 2 // integer division floors and remains uint
}
