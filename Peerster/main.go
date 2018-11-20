package main

import(
	"fmt"
	"flag"
	"protobuf"
	//"math/rand"
	"mux"
    "handlers"
    "net/http"
    "bytes"
    "strings"
)

func listenUIPort() {

	defer gossiper.UIPortConn.Close()
 	
    packetBytes := make([]byte, UDP_PACKET_SIZE)
 
    for true {

        _,_,err := gossiper.UIPortConn.ReadFromUDP(packetBytes)
        isError(err)

        receivedPkt := &ClientPacket{} 
		protobuf.Decode(packetBytes, receivedPkt)

        if(receivedPkt.Message != nil) {

	        msg := receivedPkt.Message.Text

	        fmt.Println("CLIENT MESSAGE", msg) 
	        fmt.Println("PEERS", gossiper.Peers_as_single_string)

	        gossiper.SafeNextClientMessageIDs.mux.Lock()

        	rumorMessage := RumorMessage{
        		Origin: gossiper.Name, 
        		ID: gossiper.SafeNextClientMessageIDs.NextClientMessageID,
        		Text: msg,
        	}

        	gossiper.SafeNextClientMessageIDs.NextClientMessageID ++
        	gossiper.SafeNextClientMessageIDs.mux.Unlock()

        	gossiper.LastRumor = rumorMessage
        	stateID := updateStatusAndRumorArray(rumorMessage, false)
        	//updateStatusAndRumorMapsWhenReceivingClientMessage(rumorMessage)

        	fmt.Println("Received client message, so sending to one of :", gossiper.Peers)

        	if(len(gossiper.Peers) > 0) {
        		go rumormongering(rumorMessage, false)
        		if(stateID != "present") {
        			fmt.Println("Error : client message", rumorMessage, "has state id :", stateID)
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

			address := getAddressFromRoutingTable(receivedPkt.Private.Destination)

			if(address != "") {
				sendPrivateMsgToSpecificPeer(privateMsg, address)
			}
		} else if(receivedPkt.File != nil) { // New file to be indexed

			filename := receivedPkt.File.FileName
			file := computeFileIndices(filename)

			if(file.Size <= MAX_FILE_SIZE) {
				// add to map of indexed files
				metahash_hex := file.Metahash

				gossiper.SafeIndexedFiles.mux.Lock()
				gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = file
				gossiper.SafeIndexedFiles.mux.Unlock()

				fmt.Println("Metahash of file indexed is :", metahash_hex)
				fmt.Println("Metafile of file indexed is :", file.Metafile)
			} else {
				fmt.Println("Error : file too big :", file.Size)
			}
		} else if(receivedPkt.Request != nil) {
			request_metahash := receivedPkt.Request.Request
			dest := receivedPkt.Request.Destination
			filename := receivedPkt.Request.FileName

			//fmt.Println("Received Request from client for :", request_metahash)

			address := getAddressFromRoutingTable(dest)

			gossiper.SafeIndexedFiles.mux.Lock()
			_, isIndexed := gossiper.SafeIndexedFiles.IndexedFiles[request_metahash]
			gossiper.SafeIndexedFiles.mux.Unlock()

			// Not already downloaded and we know the address
			if(!isIndexed && address != "") {

				gossiper.SafeIndexedFiles.mux.Lock()
				gossiper.SafeIndexedFiles.IndexedFiles[request_metahash] = File{
				    Name: filename,
				    Metafile : "",
				    Metahash: request_metahash,
				}
				gossiper.SafeIndexedFiles.mux.Unlock()

				dataRequest := DataRequest {
					Origin : gossiper.Name,
					Destination : dest,
					HopLimit : 10,
					HashValue : hexToBytes(request_metahash),
				}

				gossiper.SafeRequestDestinationToFileAndIndexes.mux.Lock()
				requestedArray, alreadyRequesting := gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[dest]

				if(!alreadyRequesting) {
					gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[dest] = []FileAndIndex{}
				}

				gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[dest] = append(requestedArray, FileAndIndex{
					Metahash : request_metahash,
					NextIndex : -1,
					Done : false,
				})

				gossiper.SafeRequestDestinationToFileAndIndexes.mux.Unlock()

				// Create an empty file with name "filename" in Downloads
				createEmptyFile(filename)

				fmt.Println("DOWNLOADING metafile of", filename, "from", dest)

				sendDataRequestToSpecificPeer(dataRequest, address)
				makeDataRequestTimer(dest, dataRequest)
			}
		} else if(receivedPkt.Search != nil) {
			keywordsAsString := receivedPkt.Search.Keywords
			receivedBudget := receivedPkt.Search.Budget

			var budget uint64
			budgetGiven := true

			//fmt.Println("File search request :", r.FormValue("Keywords"), " with budget :", r.FormValue("Budget"))

			if(receivedBudget == 0) {
				budgetGiven = false
				budget = 2
			} else {
				budget = uint64(receivedBudget)
			}

			keywords := strings.Split(keywordsAsString, ",")

			// Send search request to up to "budget" neighbours :
			nb_peers := uint64(len(gossiper.Peers))

			if(nb_peers > 0) {
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
			}
		}
    }
}

func listenGossipPort() {

	defer gossiper.GossipPortConn.Close()
	packetBytes := make([]byte, UDP_PACKET_SIZE)
	var tableUpdated bool
	var stateID string

	for true {	

        _,addr,err := gossiper.GossipPortConn.ReadFromUDP(packetBytes)
        isError(err)

		receivedPkt := &GossipPacket{} 
		protobuf.Decode(packetBytes, receivedPkt)

		peerAddr := addr.String()		
		updatePeerList(peerAddr)

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
				stateID = updateStatusAndRumorArray(rumorMessage, false)
				if(stateID == "present") {
					fmt.Println("RUMOR origin", origin, "from", peerAddr, "ID", id, "contents", msg) 
        			fmt.Println("PEERS", gossiper.Peers_as_single_string)
				}
        	} else {
        		stateID = updateStatusAndRumorArray(rumorMessage, true)
        	}

        	tableUpdated = false

        	if(stateID == "present" || stateID == "future") {
	        	tableUpdated = updateRoutingTable(rumorMessage, peerAddr)
	        }

        	if(tableUpdated) {
        		fmt.Println("DSDV", origin, peerAddr)
        	}

        	sendStatusMsgToSpecificPeer(peerAddr)
        	
        	if(len(gossiper.Peers) > 0) {
        		if(stateID == "present") {
        			go rumormongering(rumorMessage, false)
        		} else {
	        		/*if(rand.Int() % 2 == 0) {
	        			go rumormongering(rumorMessage, true)
	        		}*/
	        	}
        	}
	    } else if(receivedPkt.Status != nil) {

	    	isResponse := isStatusResponse(peerAddr)
	    	
	    	if(isResponse) {
	    		removeFinishedTimer(peerAddr)
	    	}

			peerStatus :=  receivedPkt.Status.Want

			printStatusReceived(peerStatus, peerAddr)
			//fmt.Println("PEERS", gossiper.Peers_as_single_string)

			rumorToSend, statusToSend := compareStatus(peerStatus, peerAddr)

			if(rumorToSend != nil) {

				sendRumorMsgToSpecificPeer(*rumorToSend, peerAddr)
				makeTimer(peerAddr, *rumorToSend)

			} else if(statusToSend != nil) {

				sendStatusMsgToSpecificPeer(peerAddr)

			} else {
				//fmt.Println("IN SYNC WITH", peerAddr)

				/*if(rand.Int() % 2 == 0) {
					// Reading rumor message map ?
					if(len(gossiper.SafeRumors.RumorMessages) > 0) {
		        		go rumormongering(gossiper.LastRumor, true)
		        	}
	        	}*/
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
            	address := getAddressFromRoutingTable(dest)

            	if(address != "" && hopLimit != 0) {
					privateMsg.HopLimit--
					sendPrivateMsgToSpecificPeer(privateMsg, address)
				}
			}
		} else if(receivedPkt.DataRequest != nil) {
			requestOrigin := receivedPkt.DataRequest.Origin
			dest := receivedPkt.DataRequest.Destination
			hopLimit := receivedPkt.DataRequest.HopLimit
			hashValue_bytes := receivedPkt.DataRequest.HashValue
			hashValue_hex := bytesToHex(hashValue_bytes)

			if(dest == gossiper.Name) {
				
				address := getAddressFromRoutingTable(requestOrigin)

				if(address != "") {
					gossiper.SafeIndexedFiles.mux.Lock()
					file, isMetaHash := gossiper.SafeIndexedFiles.IndexedFiles[hashValue_hex]
					gossiper.SafeIndexedFiles.mux.Unlock()

					fmt.Println("Received request :", hashValue_hex, " from :", requestOrigin)
					
					// check if hashValue is a metahash, if yes send the metafile
					if(isMetaHash) {
						//fmt.Println("THIS IS A METAHASH REQUEST")
						metahash_hex := hashValue_hex

						// add to map RequestOriginToFileAndIndex
						gossiper.SafeRequestOriginToFileAndIndexes.mux.Lock()
						filesBeingDownloaded, isDownloading := gossiper.SafeRequestOriginToFileAndIndexes.RequestOriginToFileAndIndex[requestOrigin]
						
						if(!isDownloading) {
							gossiper.SafeRequestOriginToFileAndIndexes.RequestOriginToFileAndIndex[requestOrigin] = []FileAndIndex{}
						}

						gossiper.SafeRequestOriginToFileAndIndexes.RequestOriginToFileAndIndex[requestOrigin] = append(filesBeingDownloaded, FileAndIndex{
							Metahash : metahash_hex,
							NextIndex : 0,
						})
						gossiper.SafeRequestOriginToFileAndIndexes.mux.Unlock()

						dataReply := DataReply{
							Origin : gossiper.Name,
							Destination : requestOrigin,
							HopLimit : 10,
							HashValue : hexToBytes(file.Metahash),
							Data : hexToBytes(file.Metafile),
						}

						//fmt.Println("Sending metafile :", file.Metafile)
						sendDataReplyToSpecificPeer(dataReply, address)

					} else {
						//fmt.Println("THIS IS A CHUNK REQUEST")

						// Check if we are already transmitting a file to the origin of the message, if not do nothing
						//gossiper.SafeRequestOriginToFileAndIndexes.mux.Lock()
						//filesBeingDownloaded, _ := gossiper.SafeRequestOriginToFileAndIndexes.RequestOriginToFileAndIndex[requestOrigin]
						//gossiper.SafeRequestOriginToFileAndIndexes.mux.Unlock()
						nextChunkHash := hashValue_hex
						
						//fmt.Println("We want the chunk hash :", nextChunkHash)
						chunkToSend, fileIsIndexed, _ := checkFilesForNextChunk(requestOrigin, nextChunkHash)

						var dataReply DataReply

						if(fileIsIndexed) {
								//fmt.Println("We have the file")
								//fmt.Println("The hash of the data we are sending :", bytesToHex(computeHash(hexToBytes(chunkToSend))))
								//fmt.Println("The data we are sending :", chunkToSend)

								//gossiper.SafeRequestOriginToFileAndIndexes.mux.Lock()
					            //gossiper.SafeRequestOriginToFileAndIndexes.RequestOriginToFileAndIndex[requestOrigin][index].NextIndex++
					            //gossiper.SafeRequestOriginToFileAndIndexes.mux.Unlock()

							dataReply = DataReply{
								Origin : gossiper.Name,
								Destination : requestOrigin,
								HopLimit : 10,
								HashValue : hashValue_bytes, // hash of i'th chunk
								Data : hexToBytes(chunkToSend), // get i'th chunk
							}
						} else {
							//fmt.Println("send empty data to show that we do not posses the file")
							dataReply = DataReply{
								Origin : gossiper.Name,
								Destination : requestOrigin,
								HopLimit : 10,
								HashValue : hashValue_bytes, 
								Data : []byte{}, // send empty data to show that we do not posses the file 
							}
						}

						//fmt.Println("Sending new chunk of hash :", nextChunkHash)
						sendDataReplyToSpecificPeer(dataReply, address)
					}
				}
			} else {

				address := getAddressFromRoutingTable(dest)

            	if(address != "" && hopLimit != 0) {
            		dataRequest := DataRequest {
						Origin : requestOrigin,
						Destination : dest,
						HopLimit : hopLimit - 1,
						HashValue : hashValue_bytes,
					}

					sendDataRequestToSpecificPeer(dataRequest, address)
				}
			}

		} else if(receivedPkt.DataReply != nil) {

			//fmt.Println("RECEIVED DATA REPLY")

			fileOrigin := receivedPkt.DataReply.Origin
			dest := receivedPkt.DataReply.Destination
			hashValue := receivedPkt.DataReply.HashValue
			hopLimit := receivedPkt.DataReply.HopLimit
			data := receivedPkt.DataReply.Data

			if(dest == gossiper.Name) {

				// check if we are expecting a reply from "fileOrigin" and cancel timer 
				isResponse := isDataResponse(fileOrigin)
	    	
		    	if(isResponse) {
		    		removeFinishedDataRequestTimer(fileOrigin)
		    	}

		    	// check that hashValue is hash of data and we are expecting a response
		    	hash := computeHash(data)

		    	if((bytes.Equal(hash, hashValue) && isResponse) && len(receivedPkt.DataReply.Data) != 0) {

		    		indexOfFile, isMetafile, nextChunkHash, isLastChunk := getNextChunkHashToRequest(fileOrigin, bytesToHex(hashValue))

		    		if(indexOfFile >= 0) {
		    			
		    			address := getAddressFromRoutingTable(fileOrigin)

		    			if(isMetafile) {

		    				metahash := hashValue
				    		metahash_hex := bytesToHex(metahash)
				    		metafile := receivedPkt.DataReply.Data
				    		metafile_hex := bytesToHex(metafile)

			    			if(len(metafile_hex) % 64 == 0) {
			    				// add fileOrigin and file to RequestDestinationToFileAndIndex
			    				gossiper.SafeRequestDestinationToFileAndIndexes.mux.Lock()
				    			gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].NextIndex = 0
				    			gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Metahash = metahash_hex
				    			gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Metafile = metafile_hex
				    			chunkIndex := gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].NextIndex + 1
				    			gossiper.SafeRequestDestinationToFileAndIndexes.mux.Unlock()

				    			gossiper.SafeIndexedFiles.mux.Lock()
				    			f := gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex]
				    			f.Metafile = bytesToHex(metafile)
				    			gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
				    			gossiper.SafeIndexedFiles.mux.Unlock()

				    			//fmt.Println("DOWNLOADING metafile of", f.Name, "from", fileOrigin)
				    			fmt.Println("DOWNLOADING", f.Name, "chunk", chunkIndex, "from", fileOrigin) // test

				    			// Request first chunk
				    			firstChunk := metafile[0:HASH_SIZE]

				    			dataRequest := DataRequest {
									Origin : gossiper.Name,
									Destination : fileOrigin,
									HopLimit : 10,
									HashValue : firstChunk,
								}
								
								sendDataRequestToSpecificPeer(dataRequest, address)
								makeDataRequestTimer(fileOrigin, dataRequest)

			    			} else {
			    				fmt.Println("Wrong metafile (not of chunks of size CHUNK_SIZE) : ", metafile_hex)
			    			}
						} else if(isLastChunk) {
							// Save the data received to the file name :
							gossiper.SafeRequestDestinationToFileAndIndexes.mux.Lock()
							metahash_hex := gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Metahash
							//chunkIndex := gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].NextIndex + 1
							gossiper.SafeRequestDestinationToFileAndIndexes.mux.Unlock()

							gossiper.SafeIndexedFiles.mux.Lock()
							f := gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex]
							gossiper.SafeIndexedFiles.mux.Unlock()

							writeChunkToFile(f.Name, data)

							//fmt.Println("DOWNLOADING", f.Name, "chunk", chunkIndex, "from", fileOrigin)

							hashValue_hex := bytesToHex(hashValue)

							// check if the hash of metafile is metahash
							if(hashValue_hex == f.Metafile[len(f.Metafile)-2*HASH_SIZE:len(f.Metafile)]) {
								fmt.Println("RECONSTRUCTED file", f.Name)
								// copy the file to _Downloads
								copyFileToDownloads(f.Name)

								// erase the file from RequestDestinationToFileAndIndex[fileOrigin]
								gossiper.SafeRequestDestinationToFileAndIndexes.mux.Lock()
								gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Done = true
								gossiper.SafeRequestDestinationToFileAndIndexes.mux.Unlock()

							} 
						} else {
							// Save the data received to the file name :
							gossiper.SafeRequestDestinationToFileAndIndexes.mux.Lock()
							metahash_hex := gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Metahash
							gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].NextIndex++
							chunkIndex := gossiper.SafeRequestDestinationToFileAndIndexes.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].NextIndex+1
							gossiper.SafeRequestDestinationToFileAndIndexes.mux.Unlock()

							gossiper.SafeIndexedFiles.mux.Lock()
							f := gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex]
							gossiper.SafeIndexedFiles.mux.Unlock()

							writeChunkToFile(f.Name, data)

							fmt.Println("DOWNLOADING", f.Name, "chunk", chunkIndex, "from", fileOrigin)
							
							// Send request for next chunk
							dataRequest := DataRequest {
								Origin : gossiper.Name,
								Destination : fileOrigin,
								HopLimit : 10,
								HashValue : hexToBytes(nextChunkHash),
							}
								
							sendDataRequestToSpecificPeer(dataRequest, address)
							makeDataRequestTimer(fileOrigin, dataRequest)

						}

		    		} else {
		    			fmt.Println("ERROR : CHUNK OF FILE NOT FOUND")
		    		}
		    	} else {
		    		fmt.Println("Error : hash of data is not hashValue of dataReply", hash, " != ", hashValue, 
		    			" or we do not expect response ? is response = ", isResponse)

		    		hashValue_hex := bytesToHex(hashValue)

		    		gossiper.SafeIndexedFiles.mux.Lock()
					_, exists := gossiper.SafeIndexedFiles.IndexedFiles[hashValue_hex]

					if(exists) {
						// we delete the file
						delete(gossiper.SafeIndexedFiles.IndexedFiles, hashValue_hex)
					}
					gossiper.SafeIndexedFiles.mux.Unlock()
		    	}
			} else {
				// Send to next in routing table 
				address := getAddressFromRoutingTable(dest)

            	if(address != "" && hopLimit != 0) {
            		dataReply := DataReply {
						Origin : fileOrigin,
						Destination : dest,
						HopLimit : hopLimit - 1,
						HashValue : hashValue,
						Data : data,
					}

					sendDataReplyToSpecificPeer(dataReply, address)
				}
			}
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

	gossiper = NewGossiper("127.0.0.1:" + *UIPort, *gossipAddr, *name, *peers)

	if(*simple == true){
		// goroutines to listen on both ports simultaneously
		go simpleListenUIPort()
		simpleListenGossipPort()
	} else { 
		// goroutines to listen on both ports, run anti entropy and serve frontend simultaneously
		if(*rtimer != 0) {
			go generatePeriodicalRouteMessage(*rtimer)
		}

		go listenUIPort()
		go listenGossipPort()
		//go antiEntropy()

		r := mux.NewRouter()

		r.HandleFunc("/id", func(w http.ResponseWriter, r *http.Request) {
		    IDHandler(w, r)
		})
		r.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		    MessageHandler(w, r)
		})
		r.HandleFunc("/node", func(w http.ResponseWriter, r *http.Request) {
		    NodeHandler(w, r)
		})
		r.HandleFunc("/route", func(w http.ResponseWriter, r *http.Request) {
		    CloseNodeHandler(w, r)
		})
		r.HandleFunc("/privateMessage", func(w http.ResponseWriter, r *http.Request) {
		    PrivateMessageHandler(w, r)
		})
		r.HandleFunc("/fileSharing", func(w http.ResponseWriter, r *http.Request) {
		    FileSharingHandler(w, r)
		})
		r.HandleFunc("/fileSearching", func(w http.ResponseWriter, r *http.Request) {
		    FileSearchHandler(w, r)
		})

	    http.ListenAndServe(":8080", handlers.CORS()(r))
	}

	/* LIST OF THINGS TO DO IN ORDER OF PRIORITY :
	1. for routing messages : map of [ip] -> (origin, lastID)
	2. Change the UIPort to receive a request (just as done with frontendHandler)

	ISSUES :
		- if the number of matches of a request is >= to ? (nb of keywords or constant 2 ?) then stop augmenting
		- Check if two keywords are the same ?

	commented rand.Int() in main.go, messageHandler of frontendHandler.go and in makeTimer of basicMethods, to uncomment.
	Should I erase the node requesting the file when I send him the last reply ? If yes after how much time
	*/

}