package main

import(
	"fmt"
	"flag"
	"protobuf"
	"math/rand"
	"mux"
    "handlers"
    "net/http"
)

func listenUIPort(gossiper *Gossiper) {

	defer gossiper.UIPortConn.Close()
 	
    packetBytes := make([]byte, 1024)
 
    for true {

        _,_,err := gossiper.UIPortConn.ReadFromUDP(packetBytes)
        isError(err)

        receivedPkt := &ClientPacket{} 
		protobuf.Decode(packetBytes, receivedPkt)

        if(receivedPkt.Message != nil) {

	        msg := receivedPkt.Message.Text

	        fmt.Println("CLIENT MESSAGE", msg) 
	        //fmt.Println("PEERS", gossiper.Peers_as_single_string)

        	rumorMessage := RumorMessage{
        		Origin: gossiper.Name, 
        		ID: gossiper.NextClientMessageID,
        		Text: msg,
        	}

        	gossiper.LastRumor = rumorMessage
        	stateID := updateStatusAndRumorArray(gossiper, rumorMessage, false)
        	gossiper.NextClientMessageID ++

        	if(len(gossiper.Peers) > 0) {
        		if(stateID == "present") {
        			go rumormongering(gossiper, rumorMessage, false)
        		} else {
	        		if(rand.Int() % 2 == 0) {
	        			go rumormongering(gossiper, rumorMessage, true)
	        		}
	        	}
        	}
		} else if(receivedPkt.Private != nil) {
			privateMsg := PrivateMessage{
                Origin : gossiper.Name,
                ID : 0,
                Text : receivedPkt.Private.Text,
                Destination : receivedPkt.Private.Destination,
                HopLimit : 10,
            }

			address := getAddressFromRoutingTable(gossiper, receivedPkt.Private.Destination)

			if(address != "") {
				sendPrivateMsgToSpecificPeer(gossiper, privateMsg, address)
			}
		} else if(receivedPkt.File != nil) { // New file to be indexed

			filename := receivedPkt.File.FileName
			file := computeFileIndices(filename)

			// add to map of indexed files
			metahash_hex := bytesToHex(file.Metahash)
			gossiper.IndexedFiles[metahash_hex] = file
			fmt.Println("Got a file :", metahash_hex)

		} else if(receivedPkt.Request != nil) {

			request_metahash := receivedPkt.Request.Request

			// TODELETE, TOCHANGE, JUST FOR TESTING PURPOSE --------------------------------------------------

			//address := getAddressFromRoutingTable(gossiper, receivedPkt.Request.Destination)
			address := "127.0.0.1:5001" // TEST
			_, isIndexed := gossiper.IndexedFiles[request_metahash]

			// Not already downloaded and we know the address
			//if(!isIndexed && address != "") {
			if(!isIndexed) { // TEST
				dataRequest := DataRequest {
					Origin : gossiper.Name,
					Destination : receivedPkt.Request.Destination,
					HopLimit : 10,
					HashValue : hexToBytes(request_metahash),
				}

				fmt.Println("Got a request :", dataRequest)
				sendDataRequestToSpecificPeer(gossiper, dataRequest, address)
				// to check, should we put destination or address?
				makeDataRequestTimer(gossiper, receivedPkt.Request.Destination, dataRequest)
			}
		}
    }
}

func listenGossipPort(gossiper *Gossiper) {

	defer gossiper.GossipPortConn.Close()
	packetBytes := make([]byte, 1024)
	var tableUpdated bool
	var stateID string

	for true {	

        _,addr,err := gossiper.GossipPortConn.ReadFromUDP(packetBytes)
        isError(err)

		receivedPkt := &GossipPacket{} 
		protobuf.Decode(packetBytes, receivedPkt)

		peerAddr := addr.String()		
		updatePeerList(gossiper, peerAddr)

        if(receivedPkt.Rumor != nil) {

	        msg :=  receivedPkt.Rumor.Text
	        origin := receivedPkt.Rumor.Origin
	        id := receivedPkt.Rumor.ID

	        rumorMessage := RumorMessage{
        		Origin: origin,
        		ID: id,
        		Text: msg,
        	}

        	gossiper.LastRumor = rumorMessage
			
			// Check to see if message is not a route rumor
			if(msg != "") {		
				stateID = updateStatusAndRumorArray(gossiper, rumorMessage, false)
        		//fmt.Println("RUMOR origin", origin, "from", peerAddr, "ID", id, "contents", msg) 
        		//fmt.Println("PEERS", gossiper.Peers_as_single_string)
        	} else {
        		stateID = updateStatusAndRumorArray(gossiper, rumorMessage, true)
        	}

        	tableUpdated = false

        	if(stateID == "present" || stateID == "future") {
	        	tableUpdated = updateRoutingTable(gossiper, rumorMessage, peerAddr)
	        }

        	if(tableUpdated) {
        		fmt.Println("DSDV", origin, peerAddr)
        	}

        	sendStatusMsgToSpecificPeer(gossiper, peerAddr)
        	
        	if(len(gossiper.Peers) > 0) {
        		if(stateID == "present") {
        			go rumormongering(gossiper, rumorMessage, false)
        		} else {
	        		if(rand.Int() % 2 == 0) {
	        			go rumormongering(gossiper, rumorMessage, true)
	        		}
	        	}
        	}
	    } else if(receivedPkt.Status != nil) {

	    	isResponse := isStatusResponse(gossiper, peerAddr)
	    	
	    	if(isResponse) {
	    		removeFinishedTimer(gossiper, peerAddr)
	    	}

			peerStatus :=  receivedPkt.Status.Want

			printStatusReceived(gossiper, peerStatus, peerAddr)
			fmt.Println("PEERS", gossiper.Peers_as_single_string)

			rumorToSend, statusToSend := compareStatus(gossiper, peerStatus, peerAddr)

			if(rumorToSend != nil) {

				sendRumorMsgToSpecificPeer(gossiper, *rumorToSend, peerAddr)
				makeTimer(gossiper, peerAddr, *rumorToSend)

			} else if(statusToSend != nil) {

				sendStatusMsgToSpecificPeer(gossiper, peerAddr)

			} else {
				fmt.Println("IN SYNC WITH", peerAddr)

				if(rand.Int() % 2 == 0) {
					// Reading rumor message map ?
					if(len(gossiper.SafeRumors.RumorMessages) > 0) {
		        		go rumormongering(gossiper, gossiper.LastRumor, true)
		        	}
	        	}
			}
		} else if(receivedPkt.Private != nil) {
			origin := receivedPkt.Private.Origin
			dest := receivedPkt.Private.Destination
			hopLimit := receivedPkt.Private.HopLimit
			msg := receivedPkt.Private.Text

			privateMsg := PrivateMessage{
	                Origin : origin,
	                ID : receivedPkt.Private.ID,
	                Text : msg,
	                Destination : dest,
	                HopLimit : hopLimit,
            	}

			if(dest == gossiper.Name) {
				fmt.Println("PRIVATE origin", origin, "hop-limit", hopLimit, "contents", msg)
				gossiper.PrivateMessages = append(gossiper.PrivateMessages, privateMsg)
			} else {
            	address := getAddressFromRoutingTable(gossiper, dest)

            	if(address != "" && hopLimit != 0) {
					privateMsg.HopLimit--
					sendPrivateMsgToSpecificPeer(gossiper, privateMsg, address)
				}
			}
		} else if(receivedPkt.DataRequest != nil) {
			origin := receivedPkt.DataRequest.Origin
			dest := receivedPkt.DataRequest.Destination
			hopLimit := receivedPkt.DataRequest.HopLimit
			hashValue_bytes := receivedPkt.DataRequest.HashValue
			hashValue_hex := bytesToHex(hashValue_bytes)

			fmt.Println("Received request : ", origin, dest, hashValue_bytes) // todelete

			if(dest == gossiper.Name) {
				fmt.Println("It's me !")
				
				// TODELETE JUST FOR TESTING PURPOSE ----------------------------------------------
				address := "127.0.0.1:5000"
				//address := getAddressFromRoutingTable(gossiper, origin)

				if(address != "") {
					file, isMetaHash := gossiper.IndexedFiles[hashValue_hex]
					// check if hashValue is a metafile, if yes send the first chunk
					if(isMetaHash) {
						metahash_hex := hashValue_hex

						// add to map nodeToFilesDownloaded
						_, isDownloading := gossiper.nodeToFilesDownloaded[origin]
						
						if(!isDownloading) {
							gossiper.nodeToFilesDownloaded[origin] = []FileAndIndex{}
						}

						gossiper.nodeToFilesDownloaded[origin] = append(gossiper.nodeToFilesDownloaded[origin], FileAndIndex{
							Metahash : metahash_hex,
							NextIndex : 0,
						})

						dataReply := DataReply{
							Origin : gossiper.Name,
							Destination : origin,
							HopLimit : 10,
							HashValue : file.Metafile[0:32], // hash of first chunk
							Data : getChunkByIndex(file.Name, 0).ByteArray, // get first chunk
						}
						fmt.Println("Sending response :", dataReply, " to :", address)
						fmt.Println("nodes to files : ", gossiper.nodeToFilesDownloaded)
						sendDataReplyToSpecificPeer(gossiper, dataReply, address)

					} else {
						// Check if we are already transmitting a file to the origin of the message
						//filesBeingDownloaded := []string{gossiper.nodeToFilesDownloaded[origin]}
						filesBeingDownloaded, isDownloading := gossiper.nodeToFilesDownloaded[origin]
						nextChunkHash := hashValue_hex
						
						if(isDownloading) {
							chunkToSend := checkFilesForHash(gossiper, filesBeingDownloaded, origin, nextChunkHash)
							dataReply := DataReply{
								Origin : gossiper.Name,
								Destination : origin,
								HopLimit : 10,
								HashValue : hashValue_bytes, // hash of first chunk
								Data : chunkToSend.ByteArray, // get first chunk
							}

							sendDataReplyToSpecificPeer(gossiper, dataReply, address)
						}
					}
				}
			} else {

				address := getAddressFromRoutingTable(gossiper, dest)

            	if(address != "" && hopLimit != 0) {
            		dataRequest := DataRequest {
						Origin : origin,
						Destination : dest,
						HopLimit : hopLimit - 1,
						HashValue : hashValue_bytes,
					}

					sendDataRequestToSpecificPeer(gossiper, dataRequest, address)
				}
			}

		} else if(receivedPkt.DataReply != nil) {
			// check if we are expecting a reply from "origin" and cancel timer 
			origin := receivedPkt.DataReply.Origin

			address := gossiper.RoutingTable[origin]

			if(address != "")
			isReponse := isDataResponse(gossiper, origin)
	    	
	    	if(isResponse) {
	    		removeFinishedDataRequestTimer(gossiper, peerAddr)
	    	}

	    	// check that hashValue is hash of data

		}  
    }
}


func main() {

	UIPort := flag.String("UIPort", "8080", "a string")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "a string")
	name := flag.String("name", "", "a string")
	peers := flag.String("peers", "", "a list of strings")
	rtimer := flag.Int("rtimer", 0, "an int")
	simple := flag.Bool("simple", false, "a bool")
	flag.Parse()

	var gossiper = NewGossiper("127.0.0.1:" + *UIPort, *gossipAddr, *name, *peers)

	if(*simple == true){
		// goroutines to listen on both ports simultaneously
		go simpleListenUIPort(gossiper)
		simpleListenGossipPort(gossiper)
	} else { 
		// goroutines to listen on both ports, run anti entropy and serve frontend simultaneously
		if(*rtimer != 0) {
			go generatePeriodicalRouteMessage(gossiper, *rtimer)
		}

		go listenUIPort(gossiper)
		listenGossipPort(gossiper)
		//antiEntropy(gossiper)

		r := mux.NewRouter()

		r.HandleFunc("/id", func(w http.ResponseWriter, r *http.Request) {
		    IDHandler(gossiper, w, r)
		})
		r.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		    MessageHandler(gossiper, w, r)
		})
		r.HandleFunc("/node", func(w http.ResponseWriter, r *http.Request) {
		    NodeHandler(gossiper, w, r)
		})
		r.HandleFunc("/route", func(w http.ResponseWriter, r *http.Request) {
		    CloseNodeHandler(gossiper, w, r)
		})
		r.HandleFunc("/privateMessage", func(w http.ResponseWriter, r *http.Request) {
		    PrivateMessageHandler(gossiper, w, r)
		})
		r.HandleFunc("/fileSharing", func(w http.ResponseWriter, r *http.Request) {
		    FileSharingHandler(gossiper, w, r)
		})

	    http.ListenAndServe(":8080", handlers.CORS()(r))
	}

	// for routing messages : map of [ip] -> (origin, lastID)
	// rumor messages to send to frontend should be a stack
	// frontend should send number of messages it has (id of last received) so that when restarting frontend you have them all

}