package main

import(
	"fmt"
	"math/rand"
	"math"
    "regexp"
    "time"
    "os"
    "sort"
)

func sendPeriodicalSearchRequest(keywordsAsString string, nb_peers uint64) {
	var budget uint64
	budget = 2
 	var ticker *time.Ticker
	ticker = time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		// while we do not reach a budget of MAX_BUDGET or a number of matches = len(keywords), send the request again every second
		for range ticker.C {

			gossiper.SafeSearchRequests.mux.Lock()
			//searchRequestInfo, searchStillExists := gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString]
			searchRequestInfo := gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString]
			gossiper.SafeSearchRequests.mux.Unlock()

			keywords := searchRequestInfo.Keywords
			nbMatches := searchRequestInfo.NbOfMatches

			/*if(!searchStillExists) {
				fmt.Println("SEARCH FINISHED") 
				return 
			}*/

			if(budget > MAX_BUDGET) {
				return
			} else if(nbMatches >= MIN_NUMBER_MATCHES) {
				fmt.Println("SEARCH FINISHED")
				return 
			} else {
				// send the search request 
				if(nb_peers > 0) {
					budgetForAll, nbPeersWithBudgetIncreased := getDistributionOfBudget(budget, nb_peers)
					//fmt.Println("Sending sendPeriodicalSearchRequest with budget for all :", budgetForAll, " nb peers with more :", nbPeersWithBudgetIncreased)
					sendSearchRequestToNeighbours(keywords, budgetForAll, nbPeersWithBudgetIncreased)
				}

				// set budget to twice the budget
				budget = budget * 2
			}
		}
	}
}

func checkFoundMatchesPeriodically(keywordsAsString string) {
 	var ticker *time.Ticker
	ticker = time.NewTicker(time.Second)
	defer ticker.Stop()

	nbTicks := 0

	for {
		// while we do not reach a budget of MAX_BUDGET or a number of matches = len(keywords), send the request again every second
		for range ticker.C {

			gossiper.SafeSearchRequests.mux.Lock()
			//searchRequestInfo, searchStillExists := gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString]
			searchRequestInfo := gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString]
			gossiper.SafeSearchRequests.mux.Unlock()

			nbMatches := searchRequestInfo.NbOfMatches

			if(nbMatches >= MIN_NUMBER_MATCHES) {
				fmt.Println("SEARCH FINISHED")
				return 
			} else if(nbTicks == 5) {
				return
			}

			nbTicks++
		}
	}
}

// SEND SEARCH REQUEST TO ALL EXCEPT THE SENDER OF THE SEARCH
func propagateSearchRequest(keywords []string, budgetForAll uint64, nbPeersWithBudgetIncreased uint64, searchOrigin string, searchOriginAddress string) {
	
	rand.Seed(time.Now().UTC().UnixNano())

	searchRequest := SearchRequest {
		Origin: searchOrigin,
		Budget: budgetForAll,
		Keywords: keywords,
	}

	searchRequestWithAugmentedBudget := SearchRequest {
		Origin: searchOrigin,
		Budget: budgetForAll + 1,
		Keywords: keywords,
	}

	nbPeers := len(gossiper.Peers)
	allCurrentPeers := make([]string, nbPeers)
	copy(allCurrentPeers, gossiper.Peers)
	
	if(searchOriginAddress != "") {
		allCurrentPeers = deleteFromArray(allCurrentPeers, searchOriginAddress)
	}

	remainingPeersToSendSearch := make([]string, len(allCurrentPeers))
	copy(remainingPeersToSendSearch, allCurrentPeers)

	if(len(remainingPeersToSendSearch) == 0) {
		fmt.Println("No peers to send request to")	
		return
	}
	var luckyPeersToSendSearch []string

	for i := 0; i < int(nbPeersWithBudgetIncreased); i++ {
		indexToSend := rand.Intn(len(remainingPeersToSendSearch))
		luckyPeer := remainingPeersToSendSearch[indexToSend]
		remainingPeersToSendSearch = append(remainingPeersToSendSearch[:indexToSend], remainingPeersToSendSearch[indexToSend+1:]...)
		luckyPeersToSendSearch = append(luckyPeersToSendSearch, luckyPeer)
	}

	if(budgetForAll > 0) {
		// set budget for all peers to basic if not lucky (not in luckyPeersToSendSearch)
		for _,p := range allCurrentPeers {
			if(contains(luckyPeersToSendSearch, p)) {
				// send search request with augmented budget :
				sendSearchRequestToSpecificPeer(searchRequestWithAugmentedBudget, p)
			
			} else {
				sendSearchRequestToSpecificPeer(searchRequest, p)
			}
		}
	} else {
		// send to lucky peers
		for _,p := range luckyPeersToSendSearch {
			sendSearchRequestToSpecificPeer(searchRequestWithAugmentedBudget, p)
		}
	}
}

func sendSearchRequestToNeighbours(keywords []string, budgetForAll uint64, nbPeersWithBudgetIncreased uint64) {
	
	rand.Seed(time.Now().UTC().UnixNano())

	searchRequest := SearchRequest {
		Origin: gossiper.Name,
		Budget: budgetForAll,
		Keywords: keywords,
	}

	searchRequestWithAugmentedBudget := SearchRequest {
		Origin: gossiper.Name,
		Budget: budgetForAll + 1,
		Keywords: keywords,
	}

	nbPeers := len(gossiper.Peers)

	allCurrentPeers := make([]string, nbPeers)
	copy(allCurrentPeers, gossiper.Peers)

	remainingPeersToSendSearch := make([]string, nbPeers)
	copy(remainingPeersToSendSearch, allCurrentPeers)

	var luckyPeersToSendSearch []string

	for i := 0; i < int(nbPeersWithBudgetIncreased); i++ {
		indexToSend := rand.Intn(len(remainingPeersToSendSearch))
		luckyPeer := remainingPeersToSendSearch[indexToSend]
		remainingPeersToSendSearch = append(remainingPeersToSendSearch[:indexToSend], remainingPeersToSendSearch[indexToSend+1:]...)
		luckyPeersToSendSearch = append(luckyPeersToSendSearch, luckyPeer)
	}

	if(budgetForAll > 0) {
		//fmt.Println("All my peers :", allCurrentPeers, "gossiper peers :", gossiper.Peers)
		// set budget for all peers to basic if not lucky (not in luckyPeersToSendSearch)
		for _,p := range allCurrentPeers {
			if(contains(luckyPeersToSendSearch, p)) {
				// send search request with augmented budget :
				//fmt.Println("Sending search request :", searchRequestWithAugmentedBudget, " to :", p)
				sendSearchRequestToSpecificPeer(searchRequestWithAugmentedBudget, p)
			
			} else {
				//fmt.Println("Sending search request :", searchRequest, " to :", p)
				sendSearchRequestToSpecificPeer(searchRequest, p)
			}
		}
	} else {
		// send to lucky peers
		for _,p := range luckyPeersToSendSearch {
			sendSearchRequestToSpecificPeer(searchRequestWithAugmentedBudget, p)
		}
	}
}

func setSearchRequestTimer(originAndKeyword OriginAndKeywordsStruct) {
	timer := time.NewTimer(time.Millisecond * 500)

	go func() {
		<-timer.C
		gossiper.SafeOriginAndKeywords.mux.Lock()
		delete(gossiper.SafeOriginAndKeywords.OriginAndKeywords, originAndKeyword)
		gossiper.SafeOriginAndKeywords.mux.Unlock()
	}()
}


func getDistributionOfBudget(budget uint64, nb_peers uint64) (uint64, uint64) {
	
	if(nb_peers == 0) {
		return 0,0
	} 

	divResult := float64(budget)/float64(nb_peers)
	allPeersMinBudget := math.Floor(divResult)
	//nbPeersWithBudgetIncreased := math.Round((divResult - float64(allPeersMinBudget)) * float64(nb_peers))
	nbPeersWithBudgetIncreased :=  math.Round(math.Mod(float64(budget), float64(nb_peers)))
	return uint64(allPeersMinBudget), uint64(nbPeersWithBudgetIncreased)
}

func getFilesWithMatchingFilenames(keyword string) []MyFileStruct {
	gossiper.SafeIndexedFiles.mux.Lock()
	indexedFiles := gossiper.SafeIndexedFiles.IndexedFiles
	gossiper.SafeIndexedFiles.mux.Unlock()

	var matchingFiles []MyFileStruct

	for _, file := range indexedFiles{ 
	    if(matchesRegex(file.Name, keyword)) {
	    	matchingFiles = append(matchingFiles, file)
	    }
	}

	return matchingFiles
}


func downloadFileWithMetahash(filename string, metahash string) {
	gossiper.SafeKeywordToInfo.mux.Lock()
	allKeywordsToInfo := gossiper.SafeKeywordToInfo.KeywordToInfo
	gossiper.SafeKeywordToInfo.mux.Unlock()

	for _, fileInfoAndNbMatches := range allKeywordsToInfo {
		for _,fileAndChunkInfo := range fileInfoAndNbMatches.FilesAndChunksInfo {
			if(fileAndChunkInfo.Metahash == metahash && fileAndChunkInfo.FoundAllChunks) {
				
				// initialize the file :
				gossiper.SafeIndexedFiles.mux.Lock()
				f := gossiper.SafeIndexedFiles.IndexedFiles[metahash] 

				if(f.Done) {
					gossiper.SafeIndexedFiles.mux.Unlock()
					return
				}

				metafile := f.Metafile

				if(metafile == "") {
					fmt.Println("ERROR : metafile is nil in downloadFileWithMetahash of requestSearching.go")
				}

				gossiper.SafeIndexedFiles.IndexedFiles[metahash] = f
				gossiper.SafeIndexedFiles.mux.Unlock()

				// request next chunk destination
				newDestination := getNextChunkDestination(metahash, 0, "")

				if(newDestination != "") {
					//fmt.Println("downloadFileWithMetahash")
					newAddress := getAddressFromRoutingTable(newDestination)

					fmt.Println("DOWNLOADING", f.Name, "chunk", f.NextIndex+1, "from", newDestination)

					// send request for first chunk
					dataRequest := DataRequest {
						Origin : gossiper.Name,
						Destination : newDestination,
						HopLimit : 10,
						HashValue : hexToBytes(metafile[:CHUNK_HASH_SIZE_IN_HEXA]),
					}

					sendDataRequestToSpecificPeer(dataRequest, newAddress)
					makeDataRequestTimer(newDestination, dataRequest)
				}
			}
		}
	}
}

func getFilename(filename string) string {
	// check if filename is used
	_, err := os.Stat("Peerster/_SharedFiles/" + filename)

    if(os.IsNotExist(err)) {
    	return filename
    } else {
    	rand.Seed(time.Now().UTC().UnixNano())
    	n := rand.Intn(LEN_ALPHA_NUM_STRING)
	    randomChar := string(ALPHA_NUM_STRING[n:n+1])
	    return getFilename(randomChar + filename)
    }
}


func requestMetafileOfHashAndDest(metahash_hex string, dest string) {

	gossiper.SafeAwaitingRequestsMetahash.mux.Lock()
	metafile_hex, exists := gossiper.SafeAwaitingRequestsMetahash.AwaitingRequestsMetahash[metahash_hex]

	// sanity check :
	if(metafile_hex != "") {
		fmt.Println("ERROR we have metafile in fileAndChunkInfoToUpdate but not in AwaitingRequestsMetahash")
	}

	if(!exists || metafile_hex == "") {
		gossiper.SafeAwaitingRequestsMetahash.AwaitingRequestsMetahash[metahash_hex] = ""
		gossiper.SafeAwaitingRequestsMetahash.mux.Unlock() 

		// send data request for metafile
		dataRequest := DataRequest {
			Origin : gossiper.Name,
			Destination : dest,
			HopLimit : 10,
			HashValue : hexToBytes(metahash_hex),
		}

		address := getAddressFromRoutingTable(dest)

		sendDataRequestToSpecificPeer(dataRequest, address)
		makeDataRequestTimer(dest, dataRequest)
	} else {
		gossiper.SafeAwaitingRequestsMetahash.mux.Unlock() 
	}
}


func updateSearchRequestsByKeywords(keywordsMatchedMap map[string]bool, searchReplyOrigin string) {
	gossiper.SafeSearchRequests.mux.Lock()
	allSearchRequestInfos := gossiper.SafeSearchRequests.SearchRequestInfo
	gossiper.SafeSearchRequests.mux.Unlock()
	
	// all keywordsAsString of our "active" researches
	for keywordsAsString := range allSearchRequestInfos {

		infoOfKeywords := allSearchRequestInfos[keywordsAsString]

		// all keywords that were changed
		for keyword := range keywordsMatchedMap {
			if(contains(infoOfKeywords.Keywords, keyword)) {

				// Update number of matches of the request if the budget was not given (else we always show all matches)
				nbMatches := infoOfKeywords.NbOfMatches
				nbMatches += 1
				infoOfKeywords.NbOfMatches = nbMatches

				gossiper.SafeSearchRequests.mux.Lock()
				gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString] = infoOfKeywords
				gossiper.SafeSearchRequests.mux.Unlock()

				gossiper.SafeKeywordToInfo.mux.Lock()
				sendMatchToFrontend(keyword, false, searchReplyOrigin)
				gossiper.SafeKeywordToInfo.mux.Unlock()
			}
		}
	}
}

func printSearchReplies(results []*SearchResult) {
	for i,res := range results {
		fmt.Println("result ", i, " : name :", (*res).FileName, " metahash :", bytesToHex((*res).MetafileHash), " chunks :", (*res).ChunkMap)
	}
}

func sendMatchToFrontend(keyword string, limitMatches bool, searchReplyOrigin string) {
	fileChunkInfoAndNbMatchesOfKeyword := gossiper.SafeKeywordToInfo.KeywordToInfo[keyword]

	gossiper.SafeIndexedFiles.mux.Lock()

	for index := range fileChunkInfoAndNbMatchesOfKeyword.FilesAndChunksInfo {

		file := fileChunkInfoAndNbMatchesOfKeyword.FilesAndChunksInfo[index]
		myFile := gossiper.SafeIndexedFiles.IndexedFiles[file.Metahash]

		if(file.FoundAllChunks && !file.AlreadyShown && !myFile.AlreadyShown) {
			myFile.AlreadyShown = true
			gossiper.SafeIndexedFiles.IndexedFiles[file.Metahash] = myFile

			file.AlreadyShown = true
			fileChunkInfoAndNbMatchesOfKeyword.FilesAndChunksInfo[index] = file
			gossiper.SafeKeywordToInfo.KeywordToInfo[keyword] = fileChunkInfoAndNbMatchesOfKeyword

			//FOUND match <filename> at <peer> metafile=<metahash> chunks=<chunk_list>
			fmt.Print("FOUND match ", file.Filename, " at ", searchReplyOrigin, " metahash=", file.Metahash, "chunks=")
			printChunks(file.ChunkOriginToIndices[searchReplyOrigin])

			fmt.Println()

			addMatchToArray(file.Metahash, file.Filename)

		} else {
			//fmt.Println("Did not find all chunks yet :", file.FoundAllChunks, "or already showed the file :", file.AlreadyShown)
		}
	}
	gossiper.SafeIndexedFiles.mux.Unlock()
}

func printChunks(listOfIndices []uint64) {
	length := len(listOfIndices)

	if(length == 0) {
		return
	}

	orderedChunkList := getOrderedChunkList(listOfIndices)

	// print list
	for i,chunkIndex := range orderedChunkList {
		if(i == length-1) {
			fmt.Print(chunkIndex)
		} else {
			fmt.Print(chunkIndex, ",")
		}
	}
}

func getOrderedChunkList(listOfIndices []uint64) []uint64 {

	orderedChunkList := make([]uint64, len(listOfIndices))

	copy(orderedChunkList, listOfIndices)

	// from https://stackoverflow.com/questions/38607733/sorting-a-uint64-slice-in-go
	sort.Slice(orderedChunkList, func(i, j int) bool { return orderedChunkList[i] < orderedChunkList[j] })

	return orderedChunkList

}

func checkIsOrdered(listOfIndices []uint64) bool {
	var lastElem uint64
	lastElem = 0

	for _,currentElem := range listOfIndices {
		if(lastElem > currentElem) {
			return false
		} 
		lastElem = currentElem
	}
	return true
}

func addMatchToArray(metahash string, name string) {
	gossiper.AllMatches = append(gossiper.AllMatches, MatchNameAndMetahash{
		Filename: name,
		Metahash: metahash,
	})
}

func getNextChunkDestination(metahash_hex string, nextChunkIndex uint64, lastSender string) string {
	gossiper.SafeKeywordToInfo.mux.Lock()
	allKeywordsToFileInfoAndNbMatches := gossiper.SafeKeywordToInfo.KeywordToInfo
	gossiper.SafeKeywordToInfo.mux.Unlock()

	var contains bool

	for _,fileInfoAndNbMatches := range allKeywordsToFileInfoAndNbMatches {
		for _, fileInfo := range fileInfoAndNbMatches.FilesAndChunksInfo {
			if(fileInfo.Metahash == metahash_hex) {
				if(lastSender != "") {
					contains = containsUint64InArray(fileInfo.ChunkOriginToIndices[lastSender], nextChunkIndex)
				
					if(contains) {
						return lastSender
					}
				}

				for origin, chunkArrayOfOrigin := range fileInfo.ChunkOriginToIndices {
					contains = containsUint64InArray(chunkArrayOfOrigin, nextChunkIndex)

					if(contains) {
						return origin
					}
				}

				return ""
			} 
		}
	}

	return ""
}

// returns metahash, metafile, next chunk to request, downloadIsDone
func getMetahashMetafileIndexOfNextChunkFromAwaitingRequests(currentHashValue_hex string) (string, string, string, uint64, bool) {
	gossiper.SafeAwaitingRequestsMetahash.mux.Lock()
	awaitingRequests := gossiper.SafeAwaitingRequestsMetahash.AwaitingRequestsMetahash
	gossiper.SafeAwaitingRequestsMetahash.mux.Unlock()

	for metahash, metafile := range awaitingRequests {
		re := regexp.MustCompile(`[a-f0-9]{64}`)
    	metafileArray := re.FindAllString(metafile, -1) // split metafile into array of 64 char chunks
    	
    	for chunkIndex, chunk := range metafileArray {
    		if(currentHashValue_hex == chunk) {

    			// check if it is the last chunk
    			if(chunkIndex == len(metafileArray) - 1) {
    				return metahash, metafile, "", uint64(chunkIndex+1), true
    			} else {
    				return metahash, metafile, metafileArray[chunkIndex + 1], uint64(chunkIndex+1), false
    			}
    		}
    	}
	}

	return "", "", "", 0, false
}

func matchesRegex(filename string, keyword string) bool {
	r := "(.*)" + keyword + "(.*)"
	match, _ := regexp.MatchString(r, filename)

	return match
}