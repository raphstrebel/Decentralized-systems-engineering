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

			if(file != File{} && file.Size <= MAX_FILE_SIZE) {
				// add to map of indexed files
				metahash_hex := file.Metahash

				gossiper.SafeIndexedFiles.mux.Lock()
				gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = file
				gossiper.SafeIndexedFiles.mux.Unlock()

				fmt.Println("Metahash of file indexed is :", metahash_hex)
				fmt.Println("Metafile of file indexed is :", file.Metafile)
			} else {
				fmt.Println("Error : file inexistent or too big :", file)
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
				    NextIndex: -1,
				}

				fmt.Println("Received new file request :", gossiper.SafeIndexedFiles.IndexedFiles[request_metahash])
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

			gossiper.SafeSearchRequests.mux.Lock()
			_,exists := gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString]

			if(!exists) {
				gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString] = SearchRequestInformation{
					Keywords: keywords,
					NbOfMatches: 0,
					BudgetIsGiven: budgetGiven,
					//KeywordToInfo: make(map[string][]FileAndChunkInformation),
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
						sendPeriodicalSearchRequest(keywordsAsString, nb_peers)
					}
				}
			} else {
				gossiper.SafeSearchRequests.mux.Unlock()
				fmt.Println("The same request was already made :", gossiper.SafeSearchRequests.SearchRequestInfo[keywordsAsString])
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

            	if(address != "" && hopLimit > 0) {
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
						nextChunkHash := hashValue_hex
						
						chunkToSend, fileIsIndexed, _ := checkFilesForNextChunk(requestOrigin, nextChunkHash)

						var dataReply DataReply

						if(fileIsIndexed) {
							dataReply = DataReply{
								Origin : gossiper.Name,
								Destination : requestOrigin,
								HopLimit : 10,
								HashValue : hashValue_bytes, // hash of i'th chunk
								Data : hexToBytes(chunkToSend), // get i'th chunk
							}
						} else {
							dataReply = DataReply{
								Origin : gossiper.Name,
								Destination : requestOrigin,
								HopLimit : 10,
								HashValue : hashValue_bytes, 
								Data : []byte{}, // send empty data to show that we do not posses the file 
							}
						}

						sendDataReplyToSpecificPeer(dataReply, address)
					}
				}
			} else {

				address := getAddressFromRoutingTable(dest)

            	if(address != "" && hopLimit > 0) {
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
				    			f.NextIndex = 0
				    			gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
				    			gossiper.SafeIndexedFiles.mux.Unlock()

				    			gossiper.SafeAwaitingRequestsMetahash.mux.Lock()
								metafile_stored_hex, searchExists := gossiper.SafeAwaitingRequestsMetahash.AwaitingRequestsMetahash[metahash_hex]
								

								// Check if we are waiting for a metafile for a search request (if yes we don't need to download all the file)
								if(searchExists && metafile_stored_hex == "") {
									gossiper.SafeAwaitingRequestsMetahash.AwaitingRequestsMetahash[metahash_hex] = metafile_hex

									// Check if we have all chunks needed, if yes update the GUI
									// TODO

									gossiper.SafeAwaitingRequestsMetahash.mux.Unlock()
								} else {
									gossiper.SafeAwaitingRequestsMetahash.mux.Unlock()

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
								}

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

							writeChunkToFile(f.Name, data)

							f.NextIndex++
							gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
							gossiper.SafeIndexedFiles.mux.Unlock()

							fmt.Println("Received last file chunk, now got :", f)

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

								gossiper.SafeIndexedFiles.mux.Lock()
								f.Done = true
								gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
								gossiper.SafeIndexedFiles.mux.Unlock()


							} else {
								fmt.Println("Error : file was not reconstructed :", f)
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

							gossiper.SafeIndexedFiles.mux.Lock()
							f.NextIndex++
							gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex] = f
							gossiper.SafeIndexedFiles.mux.Unlock()

							//fmt.Println("Received some file chunk, now got :", f)

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
		} else if(receivedPkt.SearchRequest != nil) {
			searchOrigin := receivedPkt.SearchRequest.Origin
			budget := receivedPkt.SearchRequest.Budget
			keywords := receivedPkt.SearchRequest.Keywords
			keywordsAsString := getKeywordsAsString(keywords)

			fmt.Println("Received search request from origin :", searchOrigin, " from address :",peerAddr , " with budget :", budget, "and keywords :", keywords)

			// We do this to keep track of requests in the last 0.5 seconds
			originAndKeyword := OriginAndKeywordsStruct {
				Origin: searchOrigin,
				Keywords: keywordsAsString,
			}

			gossiper.SafeOriginAndKeywords.mux.Lock()
			_, alreadyExists := gossiper.SafeOriginAndKeywords.OriginAndKeywords[originAndKeyword]
			gossiper.SafeOriginAndKeywords.OriginAndKeywords[originAndKeyword] = true
			gossiper.SafeOriginAndKeywords.mux.Unlock()

			address := getAddressFromRoutingTable(searchOrigin)

			if(address != "" && budget > 0 && !alreadyExists) {

				// set timer for the request
				setSearchRequestTimer(originAndKeyword)

				searchReply := SearchReply{
					Origin: gossiper.Name,
					Destination: searchOrigin,
					HopLimit: 10,
					Results: []*SearchResult{},
				}

				var newResult *SearchResult
				var matchingFiles []File
				var chunkMap []uint64

				// Check all indexed file names for keywords
				for _,keyword := range keywords {
					// for every keywords, check if any file in SafeIndexedFiles has a name that matches the search
					matchingFiles = getFilesWithMatchingFilenames(keyword)

					// if any file matches the search, make a new search result to add to the search reply
					for _,file := range matchingFiles {
						
						chunkMap = getChunkMap(file)

						newResult = &SearchResult {
							FileName: file.Name,
							MetafileHash: hexToBytes(file.Metahash),
							ChunkMap: chunkMap,
						}

						searchReply.Results = append(searchReply.Results, newResult)
					}
				}

				sendSearchReplyToSpecificPeer(searchReply, address)

				// decrease budget, then send to all peers (except the origin of the search) by setting a budget as evenly distributed as possible
				budget--
				nb_peers := len(gossiper.Peers)

				if(budget > 0 && nb_peers > 1) {
					budgetForAll, nbPeersWithBudgetIncreased := getDistributionOfBudget(budget, uint64(nb_peers-1))
					propagateSearchRequest(keywords, budgetForAll, nbPeersWithBudgetIncreased, searchOrigin, address)
				}
			} else {
				fmt.Println("Either address is not found :", address, ", budget is 0 :", budget, " or the search was already done in the last 0.5sec :", alreadyExists)
			}
		} else if(receivedPkt.SearchReply != nil) {
			fmt.Println("Received search reply :", receivedPkt.SearchReply)

			searchReplyOrigin := receivedPkt.SearchReply.Origin
			dest := receivedPkt.SearchReply.Destination
			hopLimit := receivedPkt.SearchReply.HopLimit
			results := receivedPkt.SearchReply.Results

			if(dest == gossiper.Name) {
				// check out the replies we got
				keywordsMatchedMap := make(map[string]bool)

				// We must check if we already have some files of the result (if yes no need to download them)
				gossiper.SafeKeywordToInfo.mux.Lock()

				// check all files received as result to see if they match one or more keywords and add them to the files of the keyword they match 
				LoopOverSearchResults:
				for _,file := range results {
					metahash_hex := bytesToHex(file.MetafileHash)

					// check if we already have the file :
					gossiper.SafeIndexedFiles.mux.Lock()
					f := gossiper.SafeIndexedFiles.IndexedFiles[metahash_hex]

					// We already have the file in our _Downloads folder 
					if(f.Done) {
						// check what loop it breaks
						break LoopOverSearchResults
					}

					gossiper.SafeIndexedFiles.mux.Unlock()

					// range over key,value of map KeywordToInfo
					for keyword,_ := range gossiper.SafeKeywordToInfo.KeywordToInfo {

						fileChunkInfoAndNbMatchesOfKeyword := gossiper.SafeKeywordToInfo.KeywordToInfo[keyword]
						fileAndChunkInfoOfKeyword := fileChunkInfoAndNbMatchesOfKeyword.FilesAndChunksInfo
						keywordMatchesFilename := matchesRegex(file.FileName, keyword)
						
						if(keywordMatchesFilename) {
							keywordsMatchedMap[keyword] = true

							alreadyContains, indexOfFile := containsFileMetahash(fileAndChunkInfoOfKeyword, metahash_hex)

							if(!alreadyContains) {
								// Must append this file to our list of files for this keyword : (we cannot increment the matches as we do not have the metafile)
								newFileAndChunkInfo := FileAndChunkInformation{
									Filename: file.FileName,
									Metahash: metahash_hex,
									//Metafile: file.,
									//NbOfChunks uint64,
									ChunkOriginToIndices: make(map[string][]uint64),
								}
								newFileAndChunkInfo.ChunkOriginToIndices[searchReplyOrigin] = file.ChunkMap

								fileAndChunkInfoOfKeyword = append(fileAndChunkInfoOfKeyword, newFileAndChunkInfo)
								
								fileChunkInfoAndNbMatchesOfKeyword.FilesAndChunksInfo = fileAndChunkInfoOfKeyword
								fileChunkInfoAndNbMatchesOfKeyword.NbOfMatches = 0
								gossiper.SafeKeywordToInfo.KeywordToInfo[keyword] = fileChunkInfoAndNbMatchesOfKeyword
							} else {
								// So our keyword already has the file, we have the index of the file in the array 
								// so we just need to update the chunkIndices with file.ChunkMap
								fileAndChunkInfoToUpdate := fileAndChunkInfoOfKeyword[indexOfFile]

								// sanity check :
								if(fileAndChunkInfoToUpdate.Metahash != metahash_hex) {
									fmt.Println("ERROR : containsFileMetahash method outputted wrong index :", indexOfFile, "for array :", fileAndChunkInfoOfKeyword, "and resulting file :", file)
								} 

								// ATTENTION : WE MIGHT HAVE GOTTEN AN EARLIER RESULT OF THE SAME ORIGIN SO APPEND THE RESULTS
								// loop over all values (chunk indices) of the search result array, add the missing ones in the chunkOriginToIndex map :
								for _, resultChunkIndex := range file.ChunkMap {
									if(!containsUint64(fileAndChunkInfoToUpdate.ChunkOriginToIndices, resultChunkIndex)) {
										_, originExists := fileAndChunkInfoToUpdate.ChunkOriginToIndices[searchReplyOrigin]

										if(originExists) {
											fileAndChunkInfoToUpdate.ChunkOriginToIndices[searchReplyOrigin] = append(fileAndChunkInfoToUpdate.ChunkOriginToIndices[searchReplyOrigin], resultChunkIndex)
										} else {
											fileAndChunkInfoToUpdate.ChunkOriginToIndices[searchReplyOrigin] = []uint64{resultChunkIndex}
										}
									}
								}

								incrementNbMatches := false

								// check if we have the metafile
								if(fileAndChunkInfoToUpdate.Metafile != "") {
									if(fileAndChunkInfoToUpdate.NbOfChunks == 0) {
										fileAndChunkInfoToUpdate.NbOfChunks = getNbChunksFromMetafile(fileAndChunkInfoToUpdate.Metafile)
									}

									// check if we have all chunks
									nbTotalChunks := getNbTotalChunks(fileAndChunkInfoToUpdate.ChunkOriginToIndices)

									if(nbTotalChunks == fileAndChunkInfoToUpdate.NbOfChunks) {
										incrementNbMatches = true
									}

								} else {
									/* REQUEST METAFILE
									 what should we do now ? 
									If we send a request it will come back as a dataReply, and going from datareply to searchreply will be tedious...
									
									Options :
									1. make a map (inside gossiper) of awaitingRequests from metahash to metafile, when we want a new metafile we make an entry "metahash"->nil
									then when receiving a metafile we first check if the metahash is in the map and if yes we make the necessary updates before deleting 
									the entry of the map (delete(map, themetahash))

									WHEN TO DELETE ENTRIES OF MAP ?
									*/

									gossiper.SafeAwaitingRequestsMetahash.mux.Lock()
									metafile_hex, exists := gossiper.SafeAwaitingRequestsMetahash.AwaitingRequestsMetahash[metahash_hex]

									// sanity check :
									if(metafile_hex != "") {
										fmt.Println("ERROR WITH METAFILE AT SEARCHREPLY LINE WITH WHAT SHOULD WE DO")
									}

									if(!exists) {
										gossiper.SafeAwaitingRequestsMetahash.AwaitingRequestsMetahash[metahash_hex] = ""

										// send data request for metafile
										dataRequest := DataRequest {
											Origin : gossiper.Name,
											Destination : dest,
											HopLimit : 10,
											HashValue : hexToBytes(metahash_hex),
										}
										address := getAddressFromRoutingTable(dest)
										sendDataRequestToSpecificPeer(dataRequest, address)
									}

									gossiper.SafeAwaitingRequestsMetahash.mux.Unlock() 
								}

								fileAndChunkInfoOfKeyword[indexOfFile] = fileAndChunkInfoToUpdate

								if(incrementNbMatches) {
									fileChunkInfoAndNbMatchesOfKeyword.NbOfMatches++

									// Update GUI if total number of matches is >= 2
									// TODO 
								}

								fileChunkInfoAndNbMatchesOfKeyword.FilesAndChunksInfo = fileAndChunkInfoOfKeyword
								gossiper.SafeKeywordToInfo.KeywordToInfo[keyword] = fileChunkInfoAndNbMatchesOfKeyword

							}
						}
					}
				}

				// Now we must check if any searchRequest can be updated, so we keep track of all keywords that have been found in the results with keywordsMatchedMap
				// If number of matches of a search is >= 2, delete the searchRequest from gossiper.SafeSearchRequest...
				updateSearchRequestsByKeywords(keywordsMatchedMap)

				gossiper.SafeKeywordToInfo.mux.Unlock()  

			} else { 
				// we are not the destination, send message to destination
				address := getAddressFromRoutingTable(dest)

	            if(address != "" && hopLimit > 0) {
	           		searchReply := SearchReply {
						Origin : searchReplyOrigin,
						Destination : dest,
						HopLimit : hopLimit - 1,
						Results : results,
					}

					sendSearchReplyToSpecificPeer(searchReply, address)
				}
			}
		} else {
			fmt.Println("ERROR : Packet of unknown type :", receivedPkt)
		}
    }
}
func updateSearchRequestsByKeywords(keywordsMatchedMap map[string]bool) {
	gossiper.SafeSearchRequests.mux.Lock()
	allSearchRequestInfos := gossiper.SafeSearchRequests.SearchRequestInfo
	gossiper.SafeSearchRequests.mux.Unlock()

	// all keywordsAsString of our "active" researches
	for keywordsAsString := range allSearchRequestInfos {
		infoOfKeywords := allSearchRequestInfos[keywordsAsString]
		// all keywords that were changed
		LoopOverMatchedKeywords:
		for keyword := range keywordsMatchedMap {
			if(contains(infoOfKeywords.Keywords, keyword)) {

				// Update number of matches of the request if the budget was not given (else we always show all matches)
				if(!infoOfKeywords.BudgetIsGiven) {
					nbMatches := infoOfKeywords.NbOfMatches
					nbMatches += gossiper.SafeKeywordToInfo.KeywordToInfo[keyword].NbOfMatches
					infoOfKeywords.NbOfMatches = nbMatches

					alreadyShown := infoOfKeywords.AlreadyShown
					allSearchRequestInfos[keywordsAsString] = infoOfKeywords
					
					if(nbMatches >= 2 && !alreadyShown) {
						fmt.Println("Found enough matches for search :", keywordsAsString, " : ", nbMatches)
						
						// send reply to frontend to show all files that can be dowloaded
						sendMatchToFrontend(gossiper.SafeKeywordToInfo.KeywordToInfo[keyword], true)

						infoOfKeywords.AlreadyShown = true
						allSearchRequestInfos[keywordsAsString] = infoOfKeywords

						/*gossiper.SafeSearchRequests.mux.Lock()
						delete(gossiper.SafeSearchRequests.SearchRequestInfo, keywordsAsString)
						gossiper.SafeSearchRequests.mux.Unlock()*/
						break LoopOverMatchedKeywords
					} 
				} else {
					sendMatchToFrontend(gossiper.SafeKeywordToInfo.KeywordToInfo[keyword], false)
				}
			}
		}
	}
}

func sendMatchToFrontend(fileChunkInfo FileChunkInfoAndNbMatches, limitMatches bool) {
	fmt.Println("Got a match :")
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
	3. Change CLI so that with flags -file=... -request=... the downloads can start (no need for dest)
	
	4. change nb of matches to just "matched" as a boolean (otherwise a file can get matched multiple times)

	ISSUES :
		- if the number of matches of a request is >= to ? (nb of keywords or constant 2 ?) then stop augmenting
		- Check if two keywords are the same ?
		- When sending a search request, should we expect an answer ? Should we set a timeout ?

	commented rand.Int() in main.go, messageHandler of frontendHandler.go and in makeTimer of basicMethods, to uncomment.
	Should I erase the node requesting the file when I send him the last reply ? If yes after how much time
	*/

}