package main

import(
	"net"
	"sync"
	"time"
)

const CHUNK_SIZE = 1024*8

type ClientPacket struct {
    Message *NormalMessage
    Private *PrivateMessage
    File *FileMessage
    Request *FileRequestMessage
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

type FileMessage struct {
    FileName string
    Request string
}

type FileRequestMessage struct {
    FileName string
    Destination string
    Request string
}

type PeerStatus struct {
	Identifier string
	NextID uint32
} 

type StatusPacket struct {
	Want []PeerStatus
}

type DataRequest struct {
	Origin string
	Destination string
	HopLimit uint32
	HashValue []byte
}

type DataReply struct {
	Origin string
	Destination string
	HopLimit uint32
	HashValue []byte
	Data []byte
}

type GossipPacket struct {
	Simple *SimpleMessage
	Rumor *RumorMessage
	Status *StatusPacket
	Private *PrivateMessage
	DataRequest *DataRequest
	DataReply *DataReply
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
	PrivateMessages []PrivateMessage
	LastRumor RumorMessage
	SafeTimers SafeTimer
	TimersBeingChanged bool
	LastRumorSentIndex int
	LastPrivateSentIndex int
	StatusOfGUI map[string]uint32
	LastNodeSentIndex int 
	SentCloseNodes []string
	NextClientMessageID uint32
	RoutingTable map[string]string
	IndexedFiles map[string]File
	SafeDataRequestTimers SafeTimer
	nodeToFilesDownloaded map[string][]FileAndIndex
}

type FileAndIndex struct {
	Metahash string
	NextIndex int
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

type Chunk struct {
    ByteArray []byte
}

type File struct {
    Name string
    Size int
    Metafile []byte
    Metahash []byte
}