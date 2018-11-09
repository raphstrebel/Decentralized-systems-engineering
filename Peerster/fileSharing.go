package main

import(
	"encoding/hex"
)

func sendRequest(metaHashHex string) {
	//metaHash, err := hex.DecodeString(metaHashHex)
	_, err := hex.DecodeString(metaHashHex)
	isError(err)


}