package main

import(
	"fmt"
	"flag"
	"protobuf"
	"math/rand"
	"mux"
    "handlers"
    "net/http"
    "bytes"
)

func listenUIPort(gossiper *Gossiper) {

	defer gossiper.UIPortConn.Close()
 	
    packetBytes := make([]byte, UDP_PACKET_SIZE)
 
    for true {

        _,_,err := gossiper.UIPortConn.ReadFromUDP(packetBytes)
        isError(err)

        receivedPkt := &ClientPacket{} 
		protobuf.Decode(packetBytes, receivedPkt)

        if(receivedPkt.Message != nil) {

	        msg := receivedPkt.Message.Text

	        //fmt.Println("CLIENT MESSAGE", msg) 
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

			if(file.Size <= MAX_FILE_SIZE) {
				// add to map of indexed files
				metahash_hex := file.Metahash
				gossiper.IndexedFiles[metahash_hex] = file

				fmt.Println("Metahash of file indexed is :", metahash_hex)
			} else {
				fmt.Println("Error : file too big :", file.Size)
			}
		} else if(receivedPkt.Request != nil) {
			request_metahash := receivedPkt.Request.Request
			dest := receivedPkt.Request.Destination
			filename := receivedPkt.Request.FileName

			address := getAddressFromRoutingTable(gossiper, dest)

			_, isIndexed := gossiper.IndexedFiles[request_metahash]

			// Not already downloaded and we know the address
			if(!isIndexed && address != "") {

				gossiper.IndexedFiles[request_metahash] = File{
				    Name: filename,
				    Metafile : "",
				    Metahash: request_metahash,
				}

				dataRequest := DataRequest {
					Origin : gossiper.Name,
					Destination : dest,
					HopLimit : 10,
					HashValue : hexToBytes(request_metahash),
				}

				requestedArray, alreadyRequesting := gossiper.RequestDestinationToFileAndIndex[dest]

				if(!alreadyRequesting) {
					gossiper.RequestDestinationToFileAndIndex[dest] = []FileAndIndex{}
				}

				gossiper.RequestDestinationToFileAndIndex[dest] = append(requestedArray, FileAndIndex{
					Metahash : request_metahash,
					NextIndex : -1,
				})

				// Create an empty file with name "filename" in Downloads
				createEmptyFile(filename)

				sendDataRequestToSpecificPeer(gossiper, dataRequest, address)
				makeDataRequestTimer(gossiper, dest, dataRequest)
			}
		}
    }
}

func listenGossipPort(gossiper *Gossiper) {

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
			//fmt.Println("PEERS", gossiper.Peers_as_single_string)

			rumorToSend, statusToSend := compareStatus(gossiper, peerStatus, peerAddr)

			if(rumorToSend != nil) {

				sendRumorMsgToSpecificPeer(gossiper, *rumorToSend, peerAddr)
				makeTimer(gossiper, peerAddr, *rumorToSend)

			} else if(statusToSend != nil) {

				sendStatusMsgToSpecificPeer(gossiper, peerAddr)

			} else {
				//fmt.Println("IN SYNC WITH", peerAddr)

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
			requestOrigin := receivedPkt.DataRequest.Origin
			dest := receivedPkt.DataRequest.Destination
			hopLimit := receivedPkt.DataRequest.HopLimit
			hashValue_bytes := receivedPkt.DataRequest.HashValue
			hashValue_hex := bytesToHex(hashValue_bytes)

			if(dest == gossiper.Name) {
				
				address := getAddressFromRoutingTable(gossiper, requestOrigin)

				if(address != "") {
					file, isMetaHash := gossiper.IndexedFiles[hashValue_hex]
					
					// check if hashValue is a metahash, if yes send the metafile
					if(isMetaHash) {
						metahash_hex := hashValue_hex

						// add to map RequestOriginToFileAndIndex
						filesBeingDownloaded, isDownloading := gossiper.RequestOriginToFileAndIndex[requestOrigin]
						
						if(!isDownloading) {
							gossiper.RequestOriginToFileAndIndex[requestOrigin] = []FileAndIndex{}
						}

						gossiper.RequestOriginToFileAndIndex[requestOrigin] = append(filesBeingDownloaded, FileAndIndex{
							Metahash : metahash_hex,
							NextIndex : 0,
						})

						dataReply := DataReply{
							Origin : gossiper.Name,
							Destination : requestOrigin,
							HopLimit : 10,
							HashValue : hexToBytes(file.Metahash),
							Data : hexToBytes(file.Metafile),
						}

						sendDataReplyToSpecificPeer(gossiper, dataReply, address)

					} else {
						// Check if we are already transmitting a file to the origin of the message, if not do nothing

						filesBeingDownloaded, isDownloading := gossiper.RequestOriginToFileAndIndex[requestOrigin]
						nextChunkHash := hashValue_hex
						
						if(isDownloading) {

							chunkToSend, fileIsIndexed := checkFilesForNextChunk(gossiper, filesBeingDownloaded, requestOrigin, nextChunkHash)
							var dataReply DataReply

							if(fileIsIndexed) {
								dataReply = DataReply{
									Origin : gossiper.Name,
									Destination : requestOrigin,
									HopLimit : 10,
									HashValue : hashValue_bytes, // hash of i'th chunk
									Data : chunkToSend, // get i'th chunk
								}
							} else {
								fmt.Println("NOT INDEXED ")
								dataReply = DataReply{
									Origin : gossiper.Name,
									Destination : requestOrigin,
									HopLimit : 10,
									HashValue : hashValue_bytes, 
									Data : []byte{}, // send empty data to show that we do not posses the file 
								}
							}

							sendDataReplyToSpecificPeer(gossiper, dataReply, address)
						}
					}
				}
			} else {

				address := getAddressFromRoutingTable(gossiper, dest)

            	if(address != "" && hopLimit != 0) {
            		dataRequest := DataRequest {
						Origin : requestOrigin,
						Destination : dest,
						HopLimit : hopLimit - 1,
						HashValue : hashValue_bytes,
					}

					sendDataRequestToSpecificPeer(gossiper, dataRequest, address)
				}
			}

		} else if(receivedPkt.DataReply != nil) {

			fileOrigin := receivedPkt.DataReply.Origin
			dest := receivedPkt.DataReply.Destination
			hashValue := receivedPkt.DataReply.HashValue
			hopLimit := receivedPkt.DataReply.HopLimit
			data := receivedPkt.DataReply.Data


			if(dest == gossiper.Name) {

				// check if we are expecting a reply from "fileOrigin" and cancel timer 
				isResponse := isDataResponse(gossiper, fileOrigin)
	    	
		    	if(isResponse) {
		    		removeFinishedDataRequestTimer(gossiper, fileOrigin)
		    	}

		    	// TODO : moodle forum : check if hashvalue is metadata and data is [] -> cancel timer and do nothing

		    	// check that hashValue is hash of data and we are expecting a response
		    	hash := computeHash(data)

		    	if((bytes.Equal(hash, hashValue) && isResponse) && len(receivedPkt.DataReply.Data) != 0) {

		    		indexOfFile, isMetafile, nextChunkHash, isLastChunk := getNextChunkHashToRequest(gossiper, fileOrigin, bytesToHex(hashValue))

		    		if(indexOfFile >= 0) {
		    			
		    			address := getAddressFromRoutingTable(gossiper, fileOrigin)

		    			if(isMetafile) {

		    				metahash := hashValue
				    		metahash_hex := bytesToHex(metahash)
				    		metafile := receivedPkt.DataReply.Data
				    		metafile_hex := bytesToHex(metafile)

			    			if(len(metafile_hex) % 64 == 0) {
			    				// add fileOrigin and file to RequestDestinationToFileAndIndex
				    			gossiper.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].NextIndex = 0
				    			gossiper.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Metahash = metahash_hex
				    			gossiper.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Metafile = metafile_hex

				    			f := gossiper.IndexedFiles[metahash_hex]
				    			f.Metafile = bytesToHex(metafile)
				    			gossiper.IndexedFiles[metahash_hex] = f

				    			// Request first chunk
				    			firstChunk := metafile[0:HASH_SIZE]

				    			dataRequest := DataRequest {
									Origin : gossiper.Name,
									Destination : fileOrigin,
									HopLimit : 10,
									HashValue : firstChunk,
								}
								
								fmt.Println("Sending request")
								sendDataRequestToSpecificPeer(gossiper, dataRequest, address)
								makeDataRequestTimer(gossiper, fileOrigin, dataRequest)

								//fmt.Println("4. Now we have the metafile: ", gossiper.RequestDestinationToFileAndIndex[fileOrigin])
			    			} else {
			    				fmt.Println("Wrong metafile (not of chunks of size CHUNK_SIZE) : ", metafile_hex)
			    			}
						} else if(isLastChunk) {
							fmt.Println("It is the last chunk")

							// Save the data received to the file name :
							metahash_hex := gossiper.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Metahash
							f := gossiper.IndexedFiles[metahash_hex]
							writeChunkToFile(f.Name, data)

							hashValue_hex := bytesToHex(hashValue)

							// check if the hash of metafile is metahash
							if(hashValue_hex == f.Metafile[len(f.Metafile)-2*HASH_SIZE:len(f.Metafile)]) {
								fmt.Println("DOWNLOAD COMPLETE")
								// copy the file to _Downloads
								copyFileToDownloads(f.Name)
								// erase the file from RequestDestinationToFileAndIndex[fileOrigin]

							} 

						} else {
							fmt.Println("Got a non metahash: ", nextChunkHash)

							// Save the data received to the file name :
							metahash_hex := gossiper.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].Metahash
							f := gossiper.IndexedFiles[metahash_hex]
							writeChunkToFile(f.Name, data)

							gossiper.RequestDestinationToFileAndIndex[fileOrigin][indexOfFile].NextIndex++
							
							// Send request for next chunk
							dataRequest := DataRequest {
								Origin : gossiper.Name,
								Destination : fileOrigin,
								HopLimit : 10,
								HashValue : hexToBytes(nextChunkHash),
							}
								
							fmt.Println("Sending next request")
							sendDataRequestToSpecificPeer(gossiper, dataRequest, address)
							makeDataRequestTimer(gossiper, fileOrigin, dataRequest)

						}

		    		} else {
		    			fmt.Println("ERROR : CHUNK OF FILE NOT FOUND")
		    		}
		    	} else {
		    		fmt.Println("Error : hash of data is not hashValue of dataReply", receivedPkt.DataReply.HashValue, " != ", 
		    			hash, " or we do not expect response ? ", !isResponse)
		    	}
			} else {
				// Send to next in routing table 
				address := getAddressFromRoutingTable(gossiper, dest)

            	if(address != "" && hopLimit != 0) {
            		dataReply := DataReply {
						Origin : fileOrigin,
						Destination : dest,
						HopLimit : hopLimit - 1,
						HashValue : hashValue,
					}

					sendDataReplyToSpecificPeer(gossiper, dataReply, address)
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
		go listenGossipPort(gossiper)
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
		r.HandleFunc("/fileSharing", func(w http.ResponseWriter, r *http.Request) {
		    FileSharingHandler(gossiper, w, r)
		})

	    http.ListenAndServe(":8080", handlers.CORS()(r))
	}

	/* LIST OF THINGS TO DO IN ORDER OF PRIORITY :

	2. Lock all maps 
	3. Test thoroughly with multiple gossipers and Thierry and Alex
	4. frontend should send number of messages it has (id of last received) so that when restarting frontend you have them all
	5. for routing messages : map of [ip] -> (origin, lastID)

	*/

}