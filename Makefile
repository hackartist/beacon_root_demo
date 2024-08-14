#!/usr/bin/make -f

build:
	rm -f ./main/verifyproof/*.bin ./main/verifyproof/*.abi ./main/beaconblockroot/*.bin ./main/beaconblockroot/*.abi 
	docker run -v ./main/beaconblockroot:/a ethereum/solc:stable -o /a --evm-version paris --abi --bin /a/BeaconBlockRoot.sol
	docker run -v ./main/verifyproof:/b ethereum/solc:stable -o /b --evm-version paris --abi --bin /b/VerifyProof.sol
	docker build -t beacon_tests .

run:
	docker container run beacon_tests

shell:
	docker container run -it beacon_tests /bin/sh
