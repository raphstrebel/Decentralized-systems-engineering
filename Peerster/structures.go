package main

import(
	"net"
	"sync"
	"time"
)

type ClientPacket struct {
    Message *NormalMessage
    Private *PrivateMessage
}

type NormalMessage struct {
    Text string
}

type SimpleMessage struct {
	OriginalName string
	RelayPeerAddr string
	Contents string
}

type RumorMessage struct {
	Origin string
	ID uint32
	Text string
}

type PrivateMessage struct {
    Origin string
    ID uint32
    Text string
    Destination string
    HopLimit uint32
}

type PeerStatus struct {
	Identifier string
	NextID uint32
} 

type StatusPacket struct {
	Want []PeerStatus
}

type GossipPacket struct {
	Simple *SimpleMessage
	Rumor *RumorMessage
	Status *StatusPacket
	Private *PrivateMessage
}

type Gossiper struct {
	UIPortAddr *net.UDPAddr
	UIPortConn *net.UDPConn
	GossipPortAddr *net.UDPAddr
	GossipPortConn *net.UDPConn
	Name string
	Peers_as_single_string string
	Peers []string
	StatusPacket *StatusPacket
	SafeRumors SafeRumor
	RumorMessages []RumorMessage
	LastRumor RumorMessage
	SafeTimers SafeTimer
	TimersBeingChanged bool
	LastRumorSentIndex int
	StatusOfGUI map[string]uint32
	LastNodeSentIndex int 
	SentCloseNodes []string
	NextClientMessageID uint32
	RoutingTable map[string]string
}

type SafeRumor struct {
	RumorMessages map[string][]RumorMessage 
	mux sync.Mutex
}

type SafeTimer struct {
	ResponseTimers map[string][]ResponseTimer
	mux sync.Mutex
}

type ResponseTimer struct {
	Timer *time.Timer
	Responder string 
}