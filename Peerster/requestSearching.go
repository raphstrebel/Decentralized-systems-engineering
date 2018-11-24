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

func matchesRegex(filename string, keyword string) bool {
	r := "(.*)" + keyword + "(.*)"
	match, _ := regexp.MatchString(r, filename)

	return match
}