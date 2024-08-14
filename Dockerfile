FROM golang:1.22.6-alpine3.20

# Install helper tools
RUN apk add --update nodejs npm git nano
# RUN npm install -g solc   
# dockerized solc has key options over solc-js from npm
# moved solidity compilation into Makefile logic instead


# Install go-ethereum tools, specifically abigen
WORKDIR $GOPATH/src/github.com/ethereum
RUN git clone https://github.com/ethereum/go-ethereum.git 
WORKDIR go-ethereum
RUN go install ./cmd/abigen

WORKDIR /usr/src/app
COPY . .

# Generate go bindings for smart contracts

WORKDIR /usr/src/app/main/beaconblockroot
RUN abigen --bin BeaconBlockRoot.bin --abi BeaconBlockRoot.abi  --pkg beaconblockroot --out BeaconBlockRoot.go

WORKDIR /usr/src/app/main/verifyproof
RUN abigen --bin VerifyProof.bin --abi VerifyProof.abi  --pkg verifyproof --out VerifyProof.go

WORKDIR /usr/src/app/main
RUN go mod tidy
RUN go install example/main

CMD ["main"]
