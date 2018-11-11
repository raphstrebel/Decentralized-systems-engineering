package main

import(
	"fmt"
	"math/rand"
	"simplejson"
    "net/http"
    "regexp"
    "reflect"
)

// Send ID to frontend
func IDHandler(gossiper *Gossiper, w http.ResponseWriter, r *http.Request) {
    json := simplejson.New()
	json.Set("ID", gossiper.Name)

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
func MessageHandler(gossiper *Gossiper, w http.ResponseWriter, r *http.Request) {
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

		rumorMessage := RumorMessage{
			Origin: gossiper.Name, 
	        ID: gossiper.NextClientMessageID,
	        Text: msg,
		}

	    gossiper.NextClientMessageID++

		stateID := updateStatusAndRumorArray(gossiper, rumorMessage, false)
		
		if(len(gossiper.Peers) > 0) {
			if(stateID == "present") {
				go rumormongering(gossiper, rumorMessage, false)
			} else {
				if(rand.Int() % 2 == 0) {
					go rumormongering(gossiper, rumorMessage, true)
		        }
			}
		}
	}
}

func PrivateMessageHandler(gossiper *Gossiper, w http.ResponseWriter, r *http.Request) {
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
		//fmt.Println("Received private message :", r.FormValue("PrivateMessage"), "to", r.FormValue("Destination"))
		dest := r.FormValue("Destination")

		privateMsg := PrivateMessage{
			Origin : gossiper.Name,
			ID : 0,
			Text : r.FormValue("PrivateMessage"),
			Destination : dest,
			HopLimit : 10,
        }

		address := getAddressFromRoutingTable(gossiper, dest)
		sendPrivateMsgToSpecificPeer(gossiper, privateMsg, address)
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

func NodeHandler(gossiper *Gossiper, w http.ResponseWriter, r *http.Request) {
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
			updatePeerList(gossiper, newNodeAddr)
		}
    }   
}

func CloseNodeHandler(gossiper *Gossiper, w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)

	if(r.FormValue("Update") == "") { // client wants to get new close nodes
        json := simplejson.New()


	    // LOCK ROUTING TABLE MAP ?


	    close_nodes := reflect.ValueOf(gossiper.RoutingTable).MapKeys()
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

        address := getAddressFromRoutingTable(gossiper, dest)
		sendPrivateMsgToSpecificPeer(gossiper, privateMsg, address)
    }   
}

func FileSharingHandler(gossiper *Gossiper, w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)

	if(r.FormValue("Update") != "") {
        filename := r.FormValue("FileName")
        file := computeFileIndices(filename)

        if(file.Size <= MAX_FILE_SIZE) {
			// add to map of indexed files
			metahash_hex := file.Metahash
			gossiper.IndexedFiles[metahash_hex] = file

			fmt.Println("Metahash of file indexed is :", metahash_hex)
		} else {
			fmt.Println("Error : file too big :", file.Size)
		}

    } else {
    	filename := r.FormValue("FileName")
    	dest := r.FormValue("Destination")
    	metahash := r.FormValue("Metahash")

		address := getAddressFromRoutingTable(gossiper, dest)

		_, isIndexed := gossiper.IndexedFiles[metahash]

		// Not already downloaded and we know the address
		if(!isIndexed && address != "" && filename != "" && dest != "" && metahash != "" ) {

			gossiper.IndexedFiles[metahash] = File{
			    Name: filename,
			    Metafile : "",
			    Metahash: metahash,
			}

			dataRequest := DataRequest {
				Origin : gossiper.Name,
				Destination : dest,
				HopLimit : 10,
				HashValue : hexToBytes(metahash),
			}

			requestedArray, alreadyRequesting := gossiper.RequestDestinationToFileAndIndex[dest]

			if(!alreadyRequesting) {
				gossiper.RequestDestinationToFileAndIndex[dest] = []FileAndIndex{}
			}

			gossiper.RequestDestinationToFileAndIndex[dest] = append(requestedArray, FileAndIndex{
				Metahash : metahash,
				NextIndex : -1,
			})

			// Create an empty file with name "filename" in Downloads
			createEmptyFile(filename)

			sendDataRequestToSpecificPeer(gossiper, dataRequest, address)
			makeDataRequestTimer(gossiper, dest, dataRequest)
		} else {
			fmt.Println("ERROR : one of the fields is nil :", filename, metahash, dest, address, " or file already indexed ?", isIndexed)
		}
    }  
}