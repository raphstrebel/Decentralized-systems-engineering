package main

import(
	"crypto/sha256"
    "encoding/binary"
    "fmt"
    "math/rand"
    "time"
    //"reflect"
)

// This method takes as parameter the tx to publish and the ADDRESS of the peer we should not send to
func broadcastTxPublishToAllPeersExcept(txPublish TxPublish, exceptPeer string) {
	for _,peer := range gossiper.Peers {
		if(peer != exceptPeer) {
			sendTxPublishToSpecificPeer(txPublish, peer)
		}
	}
}

func broadcastBlockPublishToAllPeersExcept(blockPublish BlockPublish, exceptPeer string) {
	for _,peer := range gossiper.Peers {
		if(peer != exceptPeer) {
			sendBlockToSpecificPeer(blockPublish, peer)
		}
	}
}

// Note : not sure if we need the gossiper.IsMining. If we only call miningProcedure once then no (we only need isMining if we use multiple concurrent miners)
func miningProcedure() {

	//var miningEmptyBlock bool
	isFirstMinedBlock := true

	// we must always mine if not already mining
	for {
		//if(!gossiper.IsMining) {

		//	gossiper.IsMining = true

			lastBlock := gossiper.LongestChainHead

			pendingBlock := Block {
				Transactions: gossiper.PendingTx,
			}

			if(lastBlock == "") {
				pendingBlock.PrevHash = byteToByte32(make([]byte, 32))

			} else {
				pendingBlock.PrevHash = byteToByte32(hexToBytes(lastBlock))
			}

			// Set miningEmptyBlock to true if we are mining nothing
			//miningEmptyBlock = len(gossiper.PendingTx) == 0

			gossiper.PendingTx = []TxPublish{}

			newBlock, elapsedTime := mineBlock(pendingBlock)//, miningEmptyBlock)

			//if(!miningEmptyBlock) {
				newBlockPublish := BlockPublish {
					Block : newBlock,
					HopLimit: BLOCK_HOP_LIMIT,
				}

				if(isFirstMinedBlock) {

					time.Sleep(5*time.Second)	

					isFirstMinedBlock = false
				} else {
					time.Sleep(2 * elapsedTime)
				}


				isValid,_ := checkBlockPoW(newBlock)


				if(isValid) {
					handleNewBlockArrival(newBlockPublish, "")
				}
			//}

		//	gossiper.IsMining = false

		//} else {
		//	return
		//}
	}
}

// add the transactions of the block to our SafeFilenamesToMetahash map
// ATTENTION : METHOD IS UNLOCKED
func addTransactionsToFilenameMetahashMap(newTransactions []TxPublish) {

	for _,tx := range newTransactions {
		gossiper.SafeFilenamesToMetahash.FilenamesToMetahash[tx.File.Name] = bytesToHex(tx.File.MetafileHash)
	}
}

func byteToByte32(b []byte) [32]byte {

	if(len(b) != 32) {
		fmt.Println("Error : length of byte array to convert to byte32 is not 32", b)
		return [32]byte{}
	}


	return [32]byte{b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7], b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15], b[16], b[17], 
		b[18], b[19], b[20], b[21], b[22], b[23], b[24], b[25], b[26], b[27], b[28], b[29], b[30], b[31]}
}

/*func byteToByte32WithPadding(b []byte) [32]byte {
	var toReturn [32]byte

	length := len(b)

	for i := 0; i < len(b); i++ {
		toReturn[i] = b[i]
	}

	if(length < 32) {
		for i := length; i < 32; i++ {
			toReturn[i] = 0
		}
	}

	return toReturn
}*/

//func mineBlock(block Block, emptyBlock bool) (Block, time.Duration) {
func mineBlock(block Block) (Block, time.Duration) {
	var newBlockTry_hex string
	nonce := make([]byte, 32)
	var i uint32

	//newBlockTry := copyBlock(block)

	// set a random initial point :
	rand.Seed(time.Now().UTC().UnixNano())
	i = rand.Uint32()

	var maxLimit uint32
	var elapsed time.Duration
	maxLimit = 1073741824 // 2^30

	for i > maxLimit {
		rand.Seed(time.Now().UTC().UnixNano())
		i = rand.Uint32()
	}

	// start recording time
	start := time.Now()

	for {
		newBlockTry_hex = getBlockHashHex(block)

		if(newBlockTry_hex[0:4] == "0000") {

			//if(!emptyBlock) {
				fmt.Println("FOUND-BLOCK", newBlockTry_hex)

				//todelete
				//fmt.Println("the block found :", block)
			//}
			
			elapsed = time.Since(start)

			//return copyBlock(block), elapsed

			return block, elapsed

			/*return Block {
				PrevHash: block.PrevHash,
				Nonce: byteToByte32(nonce),
				Transactions: block.Transactions,
			}, elapsed*/
		}

		i++

		// increment nonce :
		binary.LittleEndian.PutUint32(nonce, i)
		block.Nonce = byteToByte32(nonce)
	}
}

/*func copyFile(f File) File {

	metahashCopy := make([]byte, 32)
	copy(metahashCopy, f.MetafileHash)

	return File {
		Name: f.Name,
		Size: f.Size,
		MetafileHash: metahashCopy,
	}
}

func copyTransactionsArray(transactions []TxPublish) []TxPublish {
	//txCopy := make([]TxPublish, len(transactions))
	txArrayCopy := []TxPublish{}
	var txCopy TxPublish

	for _,tx := range transactions {
		
		txCopy = TxPublish{
			File: copyFile(tx.File),
			HopLimit: tx.HopLimit,
		}

		txArrayCopy = append(txArrayCopy, txCopy)
	} 

	return txArrayCopy
}

func copyBlock(block Block) Block {

	prevHash := make([]byte, 32)
	nonce := make([]byte, 32)

	tx := copyTransactionsArray(block.Transactions)

	copy(prevHash, block.PrevHash[:])
	copy(nonce, block.Nonce[:])
	copy(tx, block.Transactions)

	prevHash32 := byteToByte32(prevHash)
	nonce32 := byteToByte32(nonce)

	return Block{
		PrevHash: prevHash32,
		Nonce: nonce32,
		Transactions: tx,
	}
}*/

func getBlockHashHex(block Block) string {

	//blockCopy := copyBlock(block)

	blockHash := (block).Hash()
	//blockHashCopy := make([]byte, 32)
	//copy(blockHashCopy, blockHash[:])

	return bytesToHex(blockHash[:])//bytesToHex(blockHashCopy[:])
}

func checkBlockPoW(block Block) (bool, string) {
	//blockHash := (&block).Hash()
	blockHash_hex := getBlockHashHex(block)

	if(blockHash_hex[0:4] == "0000") {
		return true, blockHash_hex
	} else {
		return false, blockHash_hex
	}
}

func checkAllFilenamesAreFree(block Block) bool {

	for _,tx := range block.Transactions {
		_, exists := gossiper.SafeFilenamesToMetahash.FilenamesToMetahash[tx.File.Name]

		if(exists) {
			return false
		}
	}

	return true
}

func addTransactionsOfForkFromBlockToBlock(newHeadHash_hex string, forkBlockHash_hex string) {
	newHeadBlock, contains := gossiper.SafeBlockchain.Blockchain[newHeadHash_hex]

	if(newHeadHash_hex == forkBlockHash_hex) {
		return
	} else if(!contains) {
		fmt.Println("ERROR : Cannot add transaction of not contained block :", newHeadHash_hex)
	} else {
		// add transactions of the newHeadBlock 
		addTransactionsToFilenameMetahashMap(newHeadBlock.Transactions)

		// Recursively call the method with the parent of newHeadBlock
		currentMainHash := make([]byte, 32)
		copy(currentMainHash, newHeadBlock.PrevHash[:])
		newHeadHash_hex = bytesToHex(currentMainHash)
		addTransactionsOfForkFromBlockToBlock(newHeadHash_hex, forkBlockHash_hex)
	}
}

func deleteTransactionsOfLongestChainFromBlockToBlock(longestChainHead_hex string, forkMainIntersection_hex string, nbRewinds int) int {
	
	if(longestChainHead_hex == GENESIS_BLOCK) {
		return nbRewinds
	}

	lastHeadBlock, contains := gossiper.SafeBlockchain.Blockchain[longestChainHead_hex]

	if(!contains) {
		fmt.Println("ERROR : Cannot delete transaction of not contained block :", longestChainHead_hex)
		return -1
	} else if(longestChainHead_hex == forkMainIntersection_hex) {
		return nbRewinds
	} else {
		// delete transactions of lastLongestChainHead_hex :
		deleteTransactionsFromFilenameToMetahash(lastHeadBlock.Transactions)

		// Recursively call the method with parent of block
		currentMainHash := make([]byte, 32)
		copy(currentMainHash, lastHeadBlock.PrevHash[:])
		longestChainHead_hex = bytesToHex(currentMainHash)

		//longestChainHead_hex = bytesToHex(lastHeadBlock.PrevHash[:])
		return deleteTransactionsOfLongestChainFromBlockToBlock(longestChainHead_hex, forkMainIntersection_hex, nbRewinds+1)
	}

	return -1
}

// ATTENTION : WE DO NOT LOCK THE FILENAMETOMETAHASH MAP HERE, ASSUME IT IS LOCKED 
func deleteTransactionsFromFilenameToMetahash(transactions []TxPublish) {
	for _,tx := range transactions {
		delete(gossiper.SafeFilenamesToMetahash.FilenamesToMetahash, tx.File.Name)
	}
}

// ATTENTION : WE DO NOT LOCK THE BLOCKCHAIN HERE, ASSUME IT IS LOCKED 
func getChainLengthFromBlock(blockHash_hex string) int {
	currentBlock, containsBlock := gossiper.SafeBlockchain.Blockchain[blockHash_hex]

	if(!containsBlock) {
		return -1
	}

	length := 0
	gotParent := true
	var parentBlock Block
	var parentHashHex string

	for gotParent {
		parentHashHex = bytesToHex(currentBlock.PrevHash[:])
		parentBlock, gotParent = gossiper.SafeBlockchain.Blockchain[parentHashHex]

		length++

		currentBlock = parentBlock
	}

	return length 
}

func getIntersectionBetweenMainAndFork(forkHeadHash_hex string) string {

	mainChainHead := gossiper.LongestChainHead

	mainChainHeadGotParent := true
	var forkHeadGotParent bool

	currentMainHash := make([]byte, 32)
	currentForkHash := make([]byte, 32)
	currentMainChainHash_hex := mainChainHead

	currentMainChainHead := gossiper.SafeBlockchain.Blockchain[currentMainChainHash_hex]
	var currentForkHead Block
	currentForkHeadHash_hex := forkHeadHash_hex

	for mainChainHeadGotParent {

		forkHeadGotParent = true

		for forkHeadGotParent {

			if(currentForkHeadHash_hex == currentMainChainHash_hex) {
				return currentMainChainHash_hex
			}

			currentForkHead = gossiper.SafeBlockchain.Blockchain[currentForkHeadHash_hex]

			copy(currentForkHash, currentForkHead.PrevHash[:])
			currentForkHeadHash_hex = bytesToHex(currentForkHash)
			currentForkHead, forkHeadGotParent = gossiper.SafeBlockchain.Blockchain[currentForkHeadHash_hex]
		}

		copy(currentMainHash, currentMainChainHead.PrevHash[:])
		currentMainChainHash_hex = bytesToHex(currentMainHash)
		currentMainChainHead, mainChainHeadGotParent = gossiper.SafeBlockchain.Blockchain[currentMainChainHash_hex]

		// restart iterating from fork head
		currentForkHeadHash_hex = forkHeadHash_hex
		currentForkHead, forkHeadGotParent = gossiper.SafeBlockchain.Blockchain[currentForkHeadHash_hex]

	}

	return ""
}

// ATTENTION : WE DO NOT LOCK THE SafeBlockchain MAP HERE, ASSUME IT IS LOCKED 
func printLongestChain() {
	
	var prevBlocHash_hex string
	
	currentBlockHash_hex := gossiper.LongestChainHead
	currentBlock := gossiper.SafeBlockchain.Blockchain[currentBlockHash_hex]
	gotParent := true
	currentHash := make([]byte, 32)

	//print : CHAIN [block-latest] [block-prev] … [block-earliest]
	fmt.Print("CHAIN")

	for gotParent {
		// print : [blockHash prevBlocHash transactions]
		copy(currentHash, currentBlock.PrevHash[:])
		prevBlocHash_hex = bytesToHex(currentHash)
		fmt.Print(" ", currentBlockHash_hex, ":", prevBlocHash_hex, ":")
		printTransactions(currentBlock.Transactions)

		currentBlock, gotParent = gossiper.SafeBlockchain.Blockchain[prevBlocHash_hex]
		currentBlockHash_hex = prevBlocHash_hex
	}

	fmt.Println()
}

func printTransactions(transactions []TxPublish) {
	
	transactionsLength := len(transactions)-1

	for i, tx := range transactions {
		if(i == transactionsLength) {
			fmt.Print(tx.File.Name)
		} else {
			fmt.Print(tx.File.Name, ",")
		}
	}
}

func (b *Block) Hash() (out [32]byte) {
	h := sha256.New()
	h.Write(b.PrevHash[:])
	h.Write(b.Nonce[:])
	binary.Write(h,binary.LittleEndian, uint32(len(b.Transactions)))
	
	for _, t := range b.Transactions {
		th := t.Hash()
		h.Write(th[:])
	}

	copy(out[:], h.Sum(nil))
	return
}

func (t *TxPublish) Hash() (out [32]byte) {
	h := sha256.New()
	binary.Write(h,binary.LittleEndian, uint32(len(t.File.Name)))
	h.Write([]byte(t.File.Name))
	h.Write(t.File.MetafileHash)
	copy(out[:], h.Sum(nil))
	return
}

/* FOR TESTING ONLY
func printEntireChain() {
	i := 1
	for head, length := range gossiper.SafeHeadsToLength.HeadToLength {
		fmt.Println("Chain", i, "of length :", length)
		printChain(head)
		i++
	}
}

// FOR TESTING ONLY
func printChain(headHash_hex string) {
	
	currentBlock, gotParent := gossiper.SafeBlockchain.Blockchain[headHash_hex]
	var prevBlocHash_hex string
	currentBlockHash_hex := headHash_hex

	//print : [block-latest] [block-prev] … [block-earliest])

	fmt.Print("CHAIN")

	for gotParent {
		// print : [blockHash prevBlocHash transactions]
		prevBlocHash_hex = bytesToHex(currentBlock.PrevHash[:])
		fmt.Print(" ", currentBlockHash_hex, " ")
		printTransactions(currentBlock.Transactions)

		currentBlock, gotParent = gossiper.SafeBlockchain.Blockchain[prevBlocHash_hex]
		currentBlockHash_hex = prevBlocHash_hex
	}

	fmt.Println()

}*/