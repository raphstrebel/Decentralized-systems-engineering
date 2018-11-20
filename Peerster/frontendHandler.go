package main

import(
	"fmt"
	"math/rand"
	"math"
	"simplejson"
    "net/http"
    "regexp"
    "reflect"
    "strings"
    "strconv"
    "time"
)

// Send ID to frontend
func IDHandler(w http.ResponseWriter, r *http.Request) {
    json := simplejson.New()
	json.Set("ID", gossiper.Name)

	gossiper.LastRumorSentIndex = -1
	gossiper.LastPrivateSentIndex = -1
	gossiper.LastNodeSentIndex = -1
	gossiper.SentCloseNodes = []string{}


	payload, err := json.MarshalJSON()
	isError(err)

	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)
   
}


/* This method does the following :
	1. Check if frontend wants to update its messages :
	2. If yes :
		3. Check the last rumor we sent : "lastRumorSentIndex"
		4. If len(gossiper.RumorMessages)-1 > lastRumorSentIndex
			5. Send all rumors from lastRumorSentIndex to len(gossiper.RumorMessages)
		6. Else do nothing
	7. If no (frontend sends a new message) :
		8. Do as in "listenUIPort"
*/
func MessageHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)

    if(r.FormValue("Update") == "") { // Send messages to frontend
    	// Send last messages, keep track of index of last one sent
	    json := simplejson.New()

	    nb_messages := len(gossiper.RumorMessages)

	    if(gossiper.LastRumorSentIndex >= nb_messages-1) {
	    	return
	    }

	    messageArray := []string{}
	    var msg string

	    for i := gossiper.LastRumorSentIndex + 1; i < nb_messages; i++ {
	    	msg = gossiper.RumorMessages[i].Origin + " : " + gossiper.RumorMessages[i].Text
	    	messageArray = append(messageArray, msg)
	    }


	    gossiper.LastRumorSentIndex = nb_messages - 1

		json.Set("Message", messageArray)

		payload, err := json.MarshalJSON()
		isError(err)

		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	} else if(r.FormValue("Message") != ""){ // Get message from frontend
		//  Do as in "listenUIPort" 

		msg := r.FormValue("Message")

		fmt.Println("CLIENT MESSAGE", msg) 	
		fmt.Println("PEERS", gossiper.Peers_as_single_string)

		gossiper.SafeNextClientMessageIDs.mux.Lock()
		rumorMessage := RumorMessage{
			Origin: gossiper.Name, 
	        ID: gossiper.SafeNextClientMessageIDs.NextClientMessageID,
	        Text: msg,
		}

	    gossiper.SafeNextClientMessageIDs.NextClientMessageID++
	    gossiper.SafeNextClientMessageIDs.mux.Unlock()

	    fmt.Println("Received client message, so sending", rumorMessage, "to one of :", gossiper.Peers)

		stateID := updateStatusAndRumorArray(rumorMessage, false)
		//updateStatusAndRumorMapsWhenReceivingClientMessage(rumorMessage)
		
		if(len(gossiper.Peers) > 0) {
			go rumormongering(rumorMessage, false)
			if(stateID != "present") {
        		fmt.Println("Error : client message", rumorMessage, "has state id :", stateID)
        	}
		}
	}
}

func PrivateMessageHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)

    if(r.FormValue("Update") == "") { // Send private messages to frontend
    	// Send last private messages, keep track of index of last one sent
	    json := simplejson.New()

	    nb_private_messages := len(gossiper.PrivateMessages)

	    if(nb_private_messages-1 <= gossiper.LastPrivateSentIndex) {
	    	return
	    }

	    privateMessageOriginArray := []string{}
	    privateMessageTextArray := []string{}

	    for i := gossiper.LastPrivateSentIndex + 1; i < nb_private_messages; i++ {
	    	privateMessageOriginArray = append(privateMessageOriginArray, gossiper.PrivateMessages[i].Origin)
	    	privateMessageTextArray = append(privateMessageTextArray, gossiper.PrivateMessages[i].Text)
	    }

	    gossiper.LastPrivateSentIndex = nb_private_messages - 1

		json.Set("PrivateMessageText", privateMessageTextArray)
		json.Set("PrivateMessageOrigin", privateMessageOriginArray)

		payload, err := json.MarshalJSON()
		isError(err)

		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	} else if(r.FormValue("PrivateMessage") != "") {
		dest := r.FormValue("Destination")

		privateMsg := PrivateMessage{
			Origin : gossiper.Name,
			ID : 0,
			Text : r.FormValue("PrivateMessage"),
			Destination : dest,
			HopLimit : 10,
        }

		address := getAddressFromRoutingTable(dest)
		sendPrivateMsgToSpecificPeer(privateMsg, address)
	}
    
}


/* This method does the follwing
	1. Check if frontend wants to update its nodes :
	2. If yes :
		3. Check the last node we sent : "LastNodeSentIndex"
		4. If len(gossiper.Peers)-1 > LastNodeSentIndex
			5. Send all nodes from LastNodeSentIndex to len(gossiper.Peers)
		6. Else do nothing
	7. If no (frontend sends a new node) :
		8. Check syntax of ip:port
		9. If syntax is ok, add to Peers and Peers_as_single_string
*/

func NodeHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)

	if(r.FormValue("Update") == "") { // Send nodes to frontend
        json := simplejson.New()
	    nodeArray := []string{}
	    nb_peers := len(gossiper.Peers)

	    if(nb_peers-1 <= gossiper.LastNodeSentIndex) {
	    	return
	    }

	    nodeArray = []string{}

	    for i := gossiper.LastNodeSentIndex + 1; i < nb_peers; i++ {
	    	nodeArray = append(nodeArray, gossiper.Peers[i])
	    }

	    gossiper.LastNodeSentIndex = nb_peers - 1

	    json.Set("Node", nodeArray)

		payload, err := json.MarshalJSON()
		isError(err)

		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)

    } else {
        newNodeAddr := r.FormValue("Node")
        // Check syntax of ip:port
        // Should match xxx.xxx.xxx.xxx:xxxxxxxxx...
        r := "^((25[0-5]|2[0-4][0-9]|[0-1][0-9]{2}|[0-9]{2}|[0-9]).)((25[0-5]|2[0-4][0-9]|[0-1][0-9]{2}|[0-9]{2}|[0-9]).)((25[0-5]|2[0-4][0-9]|"
		r = r + "[0-1][0-9]{2}|[0-9]{2}|[0-9]).)((25[0-5]|2[0-4][0-9]|[0-1][0-9]{2}|[0-9]{2}|[0-9]):)([0-9]+)$"
		match, _ := regexp.MatchString(r, newNodeAddr)

		if(match) {
			updatePeerList(newNodeAddr)
		}
    }   
}

func CloseNodeHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)

	if(r.FormValue("Update") == "") { // client wants to get new close nodes
        json := simplejson.New()

        gossiper.SafeRoutingTables.mux.Lock()
	    close_nodes := reflect.ValueOf(gossiper.SafeRoutingTables.RoutingTable).MapKeys()
	    gossiper.SafeRoutingTables.mux.Unlock()

	    nb_close_nodes := len(close_nodes)
	    nb_close_nodes_sent := len(gossiper.SentCloseNodes)

	    if(nb_close_nodes <= nb_close_nodes_sent) {
	    	return
	    }

	    closeNodesArray := []string{}
	    var n_str string

    	for _,n := range close_nodes {
    		n_str = n.Interface().(string)
    		if(!contains(gossiper.SentCloseNodes, n_str)) {
    			gossiper.SentCloseNodes = append(gossiper.SentCloseNodes, n_str)
    			closeNodesArray = append(closeNodesArray, n_str)
    		} 
    	}

	    json.Set("CloseNode", closeNodesArray)

		payload, err := json.MarshalJSON()
		isError(err)

		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)

    } else {
        dest := r.FormValue("CloseNode")
        msg := r.FormValue("Message")

        // Write new private message to close node
        privateMsg := PrivateMessage{
			Origin : gossiper.Name,
			ID : 0,
			Text : msg,
			Destination : dest,
			HopLimit : 10,
		}

        address := getAddressFromRoutingTable(dest)
		sendPrivateMsgToSpecificPeer(privateMsg, address)
    }   
}

func FileSharingHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)

	if(r.FormValue("Update") != "") {
        filename := r.FormValue("FileName")
        file := computeFileIndices(filename)

        fmt.Println("Metahash of file indexed is :", file.Metahash)
		fmt.Println("Metafile of file indexed is :", file.Metafile)

        if(file.Size <= MAX_FILE_SIZE) {
			// add to map of indexed files
			metahash_hex := file.Metahash
			gossiper.SafeIndexedFiles.mux.Lock()
			gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = file
			gossiper.SafeIndexedFiles.mux.Unlock()
		} else {
			fmt.Println("Error : file too big :", file.Size)
		}

    } else {
    	filename := r.FormValue("FileName")
    	dest := r.FormValue("Destination")
    	metahash := r.FormValue("Metahash")

		address := getAddressFromRoutingTable(dest)

		gossiper.SafeIndexedFiles.mux.Lock()
		_, isIndexed := gossiper.SafeIndexedFiles.IndexedFiles[metahash]
		gossiper.SafeIndexedFiles.mux.Unlock()

		// Not already downloaded and we know the address
		if(!isIndexed && address != "" && filename != "" && dest != "" && metahash != "" ) {

			gossiper.SafeIndexedFiles.mux.Lock()
			gossiper.SafeIndexedFiles.IndexedFiles[metahash] = File{
			    Name: filename,
			    Metafile : "",
			    Metahash: metahash,
			}
			gossiper.SafeIndexedFiles.mux.Unlock()

			dataRequest := DataRequest {
				Origin : gossiper.Name,
				Destination : dest,
				HopLimit : 10,
				HashValue : hexToBytes(metahash),
			}

			gossiper.SafeRequestDestinationToFileAndIndexes.mux.Lock()
			requestedArray, alreadyRequesting := gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[dest]

			if(!alreadyRequesting) {
				gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[dest] = []FileAndIndex{}
			}

			gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[dest] = append(requestedArray, FileAndIndex{
				Metahash : metahash,
				NextIndex : -1,
			})
			gossiper.SafeRequestDestinationToFileAndIndexes.mux.Unlock()

			// Create an empty file with name "filename" in Downloads
			createEmptyFile(filename)

			sendDataRequestToSpecificPeer(dataRequest, address)
			makeDataRequestTimer(dest, dataRequest)
		} else {
			fmt.Println("ERROR : one of the fields is nil :", filename, metahash, dest, address, " or file already indexed ?", isIndexed)
		}
    }  
}

/*
	Frontend sends a new file search request with an array of keywords and an optional budget
	Frontend periodically requests if something has been found (?)

	WHAT SHOULD WE DO IF WE RECEIVE A REQUEST AND WE HAVE SOME FILES THAT MATCH THE KEYWORDS ? (send to frontend?)
*/
func FileSearchHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	keywordsAsString := r.FormValue("Keywords")
	receivedBudget := r.FormValue("Budget")

	var budget uint64
	budgetGiven := true

	//fmt.Println("File search request :", r.FormValue("Keywords"), " with budget :", r.FormValue("Budget"))

	if(receivedBudget == "") {
		budgetGiven = false
		budget = 2
	} else {
		i, err := strconv.Atoi(receivedBudget)
		isError(err)

		budget = uint64(i)
	}

	keywords := strings.Split(keywordsAsString, ",")

	// Check if two keywords are the same ?



	// Send search request to up to "budget" neighbours :
	nb_peers := uint64(len(gossiper.Peers))

	if(nb_peers == 0) {
		fmt.Println("No peers")
		return 
	}

	// send to all peers by setting a budget as evenly distributed as possible
	budgetForAll, nbPeersWithBudgetIncreased := getDistributionOfBudget(budget, nb_peers)

	fmt.Println("All peers get budget :", budgetForAll, "some peers have increased budget :", nbPeersWithBudgetIncreased)

	total := uint64(budgetForAll*nb_peers + nbPeersWithBudgetIncreased)
	
	// Sanity check
	if(total != budget) {
		fmt.Println("ERROR : the total budget allocated and the total budget received is not the same :", total, " != ", budget)
	}

	if(budgetGiven) {
		sendSearchRequestToNeighbours(keywords, budgetForAll, nbPeersWithBudgetIncreased)
	} else {
		sendPeriodicalSearchRequest(keywordsAsString, keywords, nb_peers)
	}



	/*for _,keyword := range keywords {
		
		// for every keywords, check if any file in SafeIndexedFiles has a name that matches the search
		checkFileNamesForWord(keyword)
	}*/
}

func sendPeriodicalSearchRequest(keywordsAsString string, keywords []string, nb_peers uint64) {
	var budget uint64
	budget = 2

	gossiper.SafeMySearchRequests.mux.Lock()
	gossiper.SafeMySearchRequests.SearchRequests[keywordsAsString] = 0
	gossiper.SafeMySearchRequests.mux.Unlock()

	// in gossiper, should make a map : requestedFileHash -> allChunksFound[]
	// if the number of matches of a request is >= to ? (nb of keywords or constant 2 ?) then stop sending

	go sendPeriodicalSearchRequestHelper(keywordsAsString, keywords, budget, nb_peers)
}

func sendPeriodicalSearchRequestHelper(keywordsAsString string, keywords []string, budget uint64, nb_peers uint64) {
	var ticker *time.Ticker
	ticker = time.NewTicker(time.Second)

	for {
		// while we do not reach a budget of MAX_BUDGET or a number of matches = len(keywords), send the request again every second
		for range ticker.C {

			gossiper.SafeMySearchRequests.mux.Lock()
			nbMatches := gossiper.SafeMySearchRequests.SearchRequests[keywordsAsString]
			gossiper.SafeMySearchRequests.mux.Unlock()

			if(budget == MAX_BUDGET || nbMatches == len(keywords)) {
				fmt.Println("STOPPING TICKER") // todelete
				ticker.Stop()
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

func getDistributionOfBudget(budget uint64, nb_peers uint64) (uint64, uint64)  {
	divResult := float64(budget)/float64(nb_peers)
	allPeersMinBudget := math.Floor(divResult)
	//nbPeersWithBudgetIncreased := math.Round((divResult - float64(allPeersMinBudget)) * float64(nb_peers))
	nbPeersWithBudgetIncreased :=  math.Round(math.Mod(float64(budget), float64(nb_peers)))
	return uint64(allPeersMinBudget), uint64(nbPeersWithBudgetIncreased)
}

/*func checkFileNamesForWord(keyword string) {
	gossiper.SafeIndexedFiles.Lock()
	for _, file := range gossiper.SafeIndexedFiles.IndexedFiles { 
	    
	    if(matchesRegex(file.Name, keyword)) {
	    	// Send search reply
	    }
	}

	gossiper.SafeIndexedFiles.Unlock()
}

func matchesRegex(filename string, keyword string) bool {
	r := "(.*)" + keyword + "(.*)"
	match, _ := regexp.MatchString(r, filename)

	return match
}*/

