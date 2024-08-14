// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.19;


interface BeaconBlockRoot {
    function get(uint block_timestamp) external view returns (string memory);
}

contract VerifyProof {
    BeaconBlockRoot private bbr;
    
    function setAddressBBR(address _address) external {
        bbr = BeaconBlockRoot(_address);
    }

    function getBeaconBlockRoot(uint timestamp) internal view returns (string memory) {
        string memory root = bbr.get(timestamp);
        return root;
    }

    function verify(uint timestamp, 
                    string memory data, 
                    string[] memory proof,
                    uint bb_index) external view returns (bool) {
        string memory targetRoot = getBeaconBlockRoot(timestamp);
        string[] memory nodes = new string[](proof.length+1);

        bytes32 hash_leaf_node = sha256(bytes(data)); 
        data = iToHex(toBytes(hash_leaf_node));

        uint node_idx = 0;
        nodes[node_idx] = data;

        for (uint256 i = 0; i < proof.length - 1; i++ ) {
            bytes memory combined;
            node_idx += 1;
            if (getBBIndexBit(bb_index, i)) { // Proof data left sibling
                combined = bytes(string.concat(proof[i], data));
            } else { // Proof data right sibling
                combined = bytes(string.concat(data, proof[i]));
            }
            bytes32 hash_tree_node = sha256(combined); 
            data = iToHex(toBytes(hash_tree_node));
            nodes[node_idx] = data;
        }
        return sha256(bytes(targetRoot)) == sha256(bytes(data));
    }


    // Private helper functions for type conversion and data manipulation

    function toBytes(bytes32 _data) private pure returns (bytes memory) {
        return abi.encodePacked(_data);
    }

    function iToHex(bytes memory buffer) private pure returns (string memory) {

        // Fixed buffer size for hexadecimal convertion
        bytes memory converted = new bytes(buffer.length * 2);

        bytes memory _base = "0123456789abcdef";

        for (uint256 i = 0; i < buffer.length; i++) {
            converted[i * 2] = _base[uint8(buffer[i]) / _base.length];
            converted[i * 2 + 1] = _base[uint8(buffer[i]) % _base.length];
        }

        return string(abi.encodePacked(converted));
    }

    function substring(string memory str, uint startIndex, uint endIndex) private pure returns (string memory) {
        bytes memory strBytes = bytes(str);
        bytes memory result = new bytes(endIndex - startIndex);
        for (uint i = startIndex; i < endIndex; i++) {
            result[i - startIndex] = strBytes[i];
        }
        return string(result);
    }

    function getBBIndexBit(uint index, uint bit) private pure returns (bool) {
        return ((index & (2 ** bit)) > 0);
    }
}
