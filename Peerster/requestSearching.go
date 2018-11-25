package main

import(
	"fmt"
	"math/rand"
	"math"
    "regexp"
    "time"
)

func sendPeriodicalSearchRequest(keywordsAsString string, nb_peers uint64) {
	var budget uint64
	budget = 2

	// in gossiper, should make a map : requestedFileHash -> allChunksFound[] 
	// and another keywordsasstring -> []requestedFileHash ?
	// if the number of matches of a request is >= to ? (nb of keywords or constant 2 ?) then stop sending

	go sendPeriodicalSearchRequestHelper(keywordsAsString, budget, nb_peers)
}

func sendPeriodicalSearchRequestHelper(keywordsAsString string, budget uint64, nb_peers uint64) {
	var ticker *time.Ticker
	ticker = time.NewTicker(time.Second)
	defer ticker.Stop()

	//done := make(chan bool, 1)

	for {
		// while we do not reach a budget of MAX_BUDGET or a number of matches = len(keywords), send the request again every second
		for range ticker.C {

			gossiper.SafeSearchRequests.mux.Lock()
			searchRequestInfo, searchStillExists := gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString]
			gossiper.SafeSearchRequests.mux.Unlock()

			keywords := searchRequestInfo.Keywords
			nbMatches := searchRequestInfo.NbOfMatches

			if(!searchStillExists) {
				fmt.Println("SEARCH STOPPED : DELETING TICKER") // todelete
				return 
			}

			if(budget > MAX_BUDGET || nbMatches >= MIN_NUMBER_MATCHES) {
				fmt.Println("STOPPING TICKER") // todelete
				//ticker.Stop()
				return 
			} else {
				// send the search request 
				if(nb_peers > 0) {
					fmt.Println("Sending new request with total budget :", budget)
					budgetForAll, nbPeersWithBudgetIncreased := getDistributionOfBudget(budget, nb_peers)
					fmt.Println("Sending new request with budget for all :", budgetForAll, "and nb peers :", nb_peers)
					sendSearchRequestToNeighbours(keywords, budgetForAll, nbPeersWithBudgetIncreased)
				}

				// set budget to twice the budget
				budget = budget * 2
			}
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

	allCurrentPeers := make([]string, len(gossiper.Peers))
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

	//fmt.Println("Peers to send request to :", remainingPeersToSendSearch)

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

				// MAKE TIMEOUT ?
			
			} else {
				sendSearchRequestToSpecificPeer(searchRequest, p)

				// MAKE TIMEOUT ?
			}
		}
	} else {
		// send to lucky peers
		for _,p := range luckyPeersToSendSearch {
			sendSearchRequestToSpecificPeer(searchRequestWithAugmentedBudget, p)

			// MAKE TIMEOUT ?
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

	allCurrentPeers := gossiper.Peers
	remainingPeersToSendSearch := make([]string, len(allCurrentPeers))
	copy(remainingPeersToSendSearch, allCurrentPeers)
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

				// MAKE TIMEOUT ?
			
			} else {
				sendSearchRequestToSpecificPeer(searchRequest, p)

				// MAKE TIMEOUT ?
			}
		}
	} else {
		// send to lucky peers
		for _,p := range luckyPeersToSendSearch {
			sendSearchRequestToSpecificPeer(searchRequestWithAugmentedBudget, p)

			// MAKE TIMEOUT ?
		}
	}
}

func setSearchRequestTimer(originAndKeyword OriginAndKeywordsStruct) {
	timer := time.NewTimer(time.Millisecond * 500)

	go func() {
		<-timer.C
		fmt.Println("search request timer expired, deleting request from map")
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

func getFilesWithMatchingFilenames(keyword string) []File {
	gossiper.SafeIndexedFiles.mux.Lock()
	indexedFiles := gossiper.SafeIndexedFiles.IndexedFiles
	gossiper.SafeIndexedFiles.mux.Unlock()

	var matchingFiles []File

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

	fmt.Println(allKeywordsToInfo)

	for _, fileInfoAndNbMatches := range allKeywordsToInfo {
		for _,fileAndChunkInfo := range fileInfoAndNbMatches.FilesAndChunksInfo {
			fmt.Println("Checking for metahash :", metahash, " : ", fileAndChunkInfo)

			if(fileAndChunkInfo.Metahash == metahash && fileAndChunkInfo.FoundAllChunks) {
				
				// initialize the file :
				gossiper.SafeIndexedFiles.mux.Lock()
				f := gossiper.SafeIndexedFiles.IndexedFiles[metahash] 

				if(f.Done) {
					gossiper.SafeIndexedFiles.mux.Unlock()
					return
				}

				f.NextIndex = 0
				f.Done = false
				metafile := f.Metafile

				if(metafile == "") {
					fmt.Println("ERROR : metafile is nil in downloadFileWithMetahash of requestSearching.go")
				}

				// ATTENTION : Should check if filename already exists before saving it

				f.Name = filename
				gossiper.SafeIndexedFiles.IndexedFiles[metahash] = f
				fmt.Println("Updated SafeIndexedFiles :", gossiper.SafeIndexedFiles.IndexedFiles)
				gossiper.SafeIndexedFiles.mux.Unlock()

				// request next chunk destination
				newDestination := getNextChunkDestination(metahash, 0, "")

				fmt.Println("The destination :", newDestination)

				if(newDestination != "") {
					newAddress := getAddressFromRoutingTable(newDestination)

					fmt.Println("DOWNLOADING", f.Name, "chunk 1", "from", newDestination, "requesting hash :", metafile[:CHUNK_HASH_SIZE_IN_HEXA])

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