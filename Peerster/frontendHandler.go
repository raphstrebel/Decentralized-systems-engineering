package main

import(
	"fmt"
	"simplejson"
    "net/http"
    "regexp"
    "reflect"
    "strings"
    "strconv"
)

// Send ID to frontend
func IDHandler(w http.ResponseWriter, r *http.Request) {
    json := simplejson.New()
	json.Set("ID", gossiper.Name)

	gossiper.LastRumorSentIndex = -1
	gossiper.LastPrivateSentIndex = -1
	gossiper.LastNodeSentIndex = -1
	gossiper.SentCloseNodes = []string{}
	gossiper.LastNodeSentIndex = -1
	gossiper.LastMatchSentIndex = -1


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
	    nb_peers := len(gossiper.Peers)

	    if(nb_peers-1 <= gossiper.LastNodeSentIndex) {
	    	return
	    }

	    nodeArray := []string{}

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
        file, isOk := computeFileIndices(filename, true)

        if(isOk) {
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
			gossiper.SafeIndexedFiles.IndexedFiles[metahash] = MyFileStruct{
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

	//fmt.Println("File search request :", r.FormValue("Keywords"), " with budget :", r.FormValue("Budget"))

	if(r.FormValue("Update") == "") { // client wants to get new matches

		json := simplejson.New()
	    matcheNamesArray := []string{}
	    matcheMetahashArray := []string{}
	    nbMatches := len(gossiper.AllMatches)

	    if(nbMatches-1 <= gossiper.LastMatchSentIndex) {
	    	return
	    }

	    for i := gossiper.LastMatchSentIndex + 1; i < nbMatches; i++ {
	    	matcheNamesArray = append(matcheNamesArray, gossiper.AllMatches[i].Filename)
	    	matcheMetahashArray = append(matcheMetahashArray, gossiper.AllMatches[i].Metahash)
	    }

	    gossiper.LastMatchSentIndex = nbMatches - 1

	    json.Set("Filename", matcheNamesArray)
		json.Set("Metahash", matcheMetahashArray)

		payload, err := json.MarshalJSON()
		isError(err)

		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	} else if(r.FormValue("Update") == "get"){

		metahash_hex := r.FormValue("Metahash")
		filename := r.FormValue("FileName")

		gossiper.SafeIndexedFiles.mux.Lock()
		f, exists := gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex]
		

		if(!exists || !f.Done) {

			f = MyFileStruct{
				Name: getFilename(filename),
				Metahash: metahash_hex,
			}

			gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
			gossiper.SafeIndexedFiles.mux.Unlock()

			// Now request the download the whole file with help of the map[origin]->chunk
			searchReplyOrigin := getNextChunkDestination(metahash_hex, 0, "")

			//fmt.Println("The search reply origin is :", searchReplyOrigin)
			
			requestMetafileOfHashAndDest(metahash_hex, searchReplyOrigin)
			//downloadFileWithMetahash(filename, metahash_hex)
		} else {
			fmt.Println("File already downloaded")

			gossiper.SafeIndexedFiles.mux.Unlock()
		}
	} else {

		keywordsAsString := r.FormValue("Keywords")
		receivedBudget := r.FormValue("Budget")

		var budget uint64
		budgetGiven := true

		//fmt.Println("Received search request from GUI")

		if(receivedBudget == "") {
			budgetGiven = false
			budget = 2
		} else {
			i, err := strconv.Atoi(receivedBudget)
			isError(err)

			budget = uint64(i)
		}

		keywords := strings.Split(keywordsAsString, ",")

		gossiper.SafeSearchRequests.mux.Lock()
		_,exists := gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString]

		if(!exists) {
			gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString] = SearchRequestInformation{
				Keywords: keywords,
				NbOfMatches: 0,
				BudgetIsGiven: budgetGiven,
			}

			gossiper.SafeSearchRequests.mux.Unlock()

			// Initialize the yet not seen keywords
			gossiper.SafeKeywordToInfo.mux.Lock()

			for _, keyword := range keywords {
				_, alreadyExists := gossiper.SafeKeywordToInfo.KeywordToInfo[keyword]
						
				if(!alreadyExists) {
					gossiper.SafeKeywordToInfo.KeywordToInfo[keyword] = FileChunkInfoAndNbMatches {
						FilesAndChunksInfo: []FileAndChunkInformation{},
						NbOfMatches: 0,
					}
				}
			}
			
			gossiper.SafeKeywordToInfo.mux.Unlock()

			// Send search request to up to "budget" neighbours :
			nb_peers := uint64(len(gossiper.Peers))

			if(nb_peers == 0) {
				return 
			}

			// send to all peers by setting a budget as evenly distributed as possible
			budgetForAll, nbPeersWithBudgetIncreased := getDistributionOfBudget(budget, nb_peers)

			total := uint64(budgetForAll*nb_peers + nbPeersWithBudgetIncreased)
			
			// Sanity check
			if(total != budget) {
				fmt.Println("ERROR : the total budget allocated and the total budget received is not the same :", total, " != ", budget)
			}

			if(budgetGiven) {
				sendSearchRequestToNeighbours(keywords, budgetForAll, nbPeersWithBudgetIncreased)
			} else {
				sendPeriodicalSearchRequest(keywordsAsString, nb_peers)
			}
		} else {
			gossiper.SafeSearchRequests.mux.Unlock()
			fmt.Println("The same request was already made :", gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString])
		}
	}
}