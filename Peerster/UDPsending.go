package main

import(
	"protobuf"
	"net"
	"fmt"
	"time"
	"math/rand"
)

func sendPacketToSpecificPeer(packet GossipPacket, address string) {
	packetBytes, err := protobuf.Encode(&packet)
	isError(err)

	// Start UDP connection
	peerAddr, err := net.ResolveUDPAddr("udp4", address)
	isError(err)
		 	
	_,err = gossiper.GossipPortConn.WriteTo(packetBytes, peerAddr)
	isError(err)
}

func sendRumorMsgToSpecificPeer(rumorMessage RumorMessage, address string) {
	//fmt.Println("MONGERING with", address)

	// Encode message
	packet := GossipPacket{Rumor: &rumorMessage}
	sendPacketToSpecificPeer(packet, address)
}

func sendPrivateMsgToSpecificPeer(privateMessage PrivateMessage, address string) {
	// Encode message
	packet := GossipPacket{Private: &privateMessage}
	sendPacketToSpecificPeer(packet, address)
}

func sendStatusMsgToSpecificPeer(address string) {
	// Encode message
	packet := GossipPacket{Status: gossiper.StatusPacket}
	sendPacketToSpecificPeer(packet, address)
}

func sendDataRequestToSpecificPeer(dataRequest DataRequest, address string) {

	// Encode message
	packet := GossipPacket{DataRequest: &dataRequest}
	sendPacketToSpecificPeer(packet, address)
}

func sendDataReplyToSpecificPeer(dataReply DataReply, address string) {
	// Encode message
	packet := GossipPacket{DataReply: &dataReply}
	sendPacketToSpecificPeer(packet, address)
}

func sendSearchRequestToSpecificPeer(searchRequest SearchRequest, address string) {
	// Encode message
	packet := GossipPacket{SearchRequest: &searchRequest}
	sendPacketToSpecificPeer(packet, address)
}

func sendSearchReplyToSpecificPeer(searchReply SearchReply, address string) {
	// Encode message
	packet := GossipPacket{SearchReply: &searchReply}
	sendPacketToSpecificPeer(packet, address)
}

func generatePeriodicalRouteMessage(rtimer int) {
	var ticker *time.Ticker
	var luckyPeer string
	var routeMessage RumorMessage
	ticker = time.NewTicker(time.Second * time.Duration(rtimer))

	for {
		for range ticker.C {
			if(len(gossiper.Peers) > 0) {

				gossiper.SafeNextClientMessageIDs.mux.Lock()
				routeMessage = RumorMessage{
					Origin: gossiper.Name,
					ID: gossiper.SafeNextClientMessageIDs.NextClientMessageID, // WHAT SHOULD THE ID OF ROUTE MESSAGES BE ?
					Text: "",
				}

				gossiper.SafeNextClientMessageIDs.NextClientMessageID++
				gossiper.SafeNextClientMessageIDs.mux.Unlock()

				rand.Seed(time.Now().UTC().UnixNano())
	    		luckyPeer = gossiper.Peers[rand.Intn(len(gossiper.Peers))]

	    		//updateStatusAndRumorMapsWhenReceivingClientMessage(routeMessage)

	    		stateID := updateStatusAndRumorArray(routeMessage, true)

	    		if(stateID != "present") {
	    			fmt.Println("Error : state of route message is :", stateID)
	    		}

	    		sendRumorMsgToSpecificPeer(routeMessage, luckyPeer)
	    		makeTimer(luckyPeer, routeMessage)
	    	}
		}
	}
}

// Anti entropy process
func antiEntropy() {
	var ticker *time.Ticker
	var luckyPeer string
	ticker = time.NewTicker(time.Second)

	for {
		for range ticker.C {
			if(len(gossiper.Peers) > 0) {
				rand.Seed(time.Now().UTC().UnixNano())
	    		luckyPeer = gossiper.Peers[rand.Intn(len(gossiper.Peers))]
	    		sendStatusMsgToSpecificPeer(luckyPeer)
	    	}
		}
	}
}

// RumorMongering process
func rumormongering(rumorMessage RumorMessage, isRandom bool) {

	rand.Seed(time.Now().UTC().UnixNano())

	if(len(gossiper.Peers) == 0) {
		return
	}

    luckyPeer := gossiper.Peers[rand.Intn(len(gossiper.Peers))]

    if(rumorMessage.Text != "") {
    	fmt.Println("sending", rumorMessage, "to", luckyPeer)
    }

    if(isRandom) {
    	fmt.Println("FLIPPED COIN sending rumor to", luckyPeer)
    }

	sendRumorMsgToSpecificPeer(rumorMessage, luckyPeer)
	makeTimer(luckyPeer, rumorMessage)
}