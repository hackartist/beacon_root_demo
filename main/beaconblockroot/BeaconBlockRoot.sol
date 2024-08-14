// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.19;

contract BeaconBlockRoot {
    uint constant HISTORY_BUFFER_LENGTH = 8191;
    mapping(uint => uint) private timestamps;
    mapping(uint => string) private block_roots; 

    function get(uint block_timestamp) external view returns (string memory) {
        uint idx = block_timestamp % HISTORY_BUFFER_LENGTH;
        uint found_ts = timestamps[idx];
        if(found_ts != block_timestamp) {
            revert();
        }

        string memory root = block_roots[idx];
        return root;
    }

    function set(uint block_timestamp, string memory block_root) external {
        uint idx = block_timestamp % HISTORY_BUFFER_LENGTH;
        timestamps[idx] = block_timestamp;
        block_roots[idx] = block_root;
    }

}
