package main

import(
	"protobuf"
	"net"
	//"fmt"
	"time"
	"math/rand"
)

func sendPacketToSpecificPeer(gossiper *Gossiper, packet GossipPacket, address string) {
	packetBytes, err := protobuf.Encode(&packet)
	isError(err)

	// Start UDP connection
	peerAddr, err := net.ResolveUDPAddr("udp4", address)
	isError(err)
		 	
	_,err = gossiper.GossipPortConn.WriteTo(packetBytes, peerAddr)
	isError(err)
}

func sendRumorMsgToSpecificPeer(gossiper *Gossiper, rumorMessage RumorMessage, address string) {
	//fmt.Println("MONGERING with", address)

	// Encode message
	packet := GossipPacket{Rumor: &rumorMessage}
	sendPacketToSpecificPeer(gossiper, packet, address)
}

func sendPrivateMsgToSpecificPeer(gossiper *Gossiper, privateMessage PrivateMessage, address string) {
	// Encode message
	packet := GossipPacket{Private: &privateMessage}
	sendPacketToSpecificPeer(gossiper, packet, address)
}

func sendStatusMsgToSpecificPeer(gossiper *Gossiper, address string) {
	// Encode message
	packet := GossipPacket{Status: gossiper.StatusPacket}
	sendPacketToSpecificPeer(gossiper, packet, address)
}

func sendDataRequestToSpecificPeer(gossiper *Gossiper, dataRequest DataRequest, address string) {
	// Encode message
	packet := GossipPacket{DataRequest: &dataRequest}
	sendPacketToSpecificPeer(gossiper, packet, address)
}

func sendDataReplyToSpecificPeer(gossiper *Gossiper, dataReply DataReply, address string) {
	// Encode message
	packet := GossipPacket{DataReply: &dataReply}
	sendPacketToSpecificPeer(gossiper, packet, address)
}

func generatePeriodicalRouteMessage(gossiper *Gossiper, rtimer int) {
	var ticker *time.Ticker
	var luckyPeer string
	var routeMessage RumorMessage
	ticker = time.NewTicker(time.Second * time.Duration(rtimer))

	for {
		for range ticker.C {
			if(len(gossiper.Peers) > 0) {
				routeMessage = RumorMessage{
					Origin: gossiper.Name,
					ID: gossiper.NextClientMessageID, // WHAT SHOULD THE ID OF ROUTE MESSAGES BE ?
					Text: "",
				}

				gossiper.NextClientMessageID++

				rand.Seed(time.Now().UTC().UnixNano())
	    		luckyPeer = gossiper.Peers[rand.Intn(len(gossiper.Peers))]
	    		sendRumorMsgToSpecificPeer(gossiper, routeMessage, luckyPeer)
	    		makeTimer(gossiper, luckyPeer, routeMessage)
	    	}
		}
	}
}

// Anti entropy process
func antiEntropy(gossiper *Gossiper) {
	var ticker *time.Ticker
	var luckyPeer string
	ticker = time.NewTicker(time.Second)

	for {
		for range ticker.C {
			if(len(gossiper.Peers) > 0) {
				rand.Seed(time.Now().UTC().UnixNano())
	    		luckyPeer = gossiper.Peers[rand.Intn(len(gossiper.Peers))]
	    		sendStatusMsgToSpecificPeer(gossiper, luckyPeer)
	    	}
		}
	}
}

// RumorMongering process
func rumormongering(gossiper *Gossiper, rumorMessage RumorMessage, isRandom bool) {

	rand.Seed(time.Now().UTC().UnixNano())

	if(len(gossiper.Peers) == 0) {
		return
	}

    luckyPeer := gossiper.Peers[rand.Intn(len(gossiper.Peers))]

    if(isRandom) {
    	//fmt.Println("FLIPPED COIN sending rumor to", luckyPeer)
    }

	sendRumorMsgToSpecificPeer(gossiper, rumorMessage, luckyPeer)
	makeTimer(gossiper, luckyPeer, rumorMessage)
}