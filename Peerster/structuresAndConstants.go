package main

import(
	"net"
	"sync"
	"time"
)

const CHUNK_SIZE = 1024*8
const UDP_PACKET_SIZE = 10000
const HASH_SIZE = 32
const MAX_FILE_SIZE = 1024*8*256 // 2MB
const CHUNK_HASH_SIZE_IN_HEXA = 64

const ALPHA_NUM_STRING = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
const LEN_ALPHA_NUM_STRING = len(ALPHA_NUM_STRING)

const MAX_BUDGET = 32
const MIN_NUMBER_MATCHES = 2

var gossiper *Gossiper

type ClientPacket struct {
    Message *NormalMessage
    Private *PrivateMessage
    File *FileMessage
    Request *FileRequestMessage
    Search *FileSearchMessage
    SearchDownload *FileDownloadMessage
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

type FileSearchMessage struct {
    Keywords string
    Budget uint64
}

type FileDownloadMessage struct {
	Filename string
	Metahash string
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

type SearchRequest struct {
	Origin string
	Budget uint64
	Keywords []string
}

type SearchReply struct {
	Origin string
	Destination string
	HopLimit uint32
	Results []*SearchResult
}

type SearchResult struct {
	FileName string
	MetafileHash []byte
	ChunkMap []uint64
	ChunkCount uint64
}

type GossipPacket struct {
	Simple *SimpleMessage
	Rumor *RumorMessage
	Status *StatusPacket
	Private *PrivateMessage
	DataRequest *DataRequest
	DataReply *DataReply
	SearchRequest *SearchRequest
	SearchReply *SearchReply
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
	LastNodeSentIndex int 
	SentCloseNodes []string
	AllMatches []MatchNameAndMetahash
	LastMatchSentIndex int
	SafeNextClientMessageIDs SafeNextClientMessageID
	SafeRoutingTables SafeRoutingTable
	SafeIndexedFiles SafeIndexedFile
	SafeDataRequestTimers SafeTimer
	SafeRequestDestinationToFileAndIndexes SafeRequestDestinationToFileAndIndex
	// map : "keywords" -> []FilesAndChunksInfo to find the chunks of the keyword search
	SafeSearchRequests SafeSearchRequest
	// Used for keeping track of requests in the last 0.5 seconds
	SafeOriginAndKeywords SafeOriginAndKeyword
	// Idea : make map of keyword to []fileandchunkInfo, nbOfMatches
	SafeKeywordToInfo SafeKeywordToInformation
	// map of metahash to metafile that are being requested
	SafeAwaitingRequestsMetahash SafeAwaitingRequestMetahash
}

type MatchNameAndMetahash struct {
	Filename string
	Metahash string
}

type SafeAwaitingRequestMetahash struct {
	// metahash as hex to metafile as hex
	AwaitingRequestsMetahash map[string]string//FileAndIndex
	mux sync.Mutex
}

type SafeSearchRequestsToNbOfMatches struct {
	SearchRequestsToNbOfMatches map[string]int
	mux sync.Mutex
}

type SafeNextClientMessageID struct {
	NextClientMessageID uint32
	mux sync.Mutex
}

type FileAndIndex struct {
	Metahash string
	Metafile string
	NextIndex int
	Done bool
}

type SafeRequestDestinationToFileAndIndex struct {
	RequestDestinationToFileAndIndex map[string][]FileAndIndex
	mux sync.Mutex
}

type SafeIndexedFile struct {
	// map of metahash as hexadecimal string to file
	IndexedFiles map[string]MyFileStruct
	mux sync.Mutex
}

type SafeRoutingTable struct {
	RoutingTable map[string]string
	mux sync.Mutex
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

type MyFileStruct struct {
    Name string
    Size int
    Metafile string
    Metahash string
    NextIndex int
    AlreadyShown bool
    Done bool
}

type SafeSearchRequest struct {
	// keywordsAsString -> all info on this request
	SearchRequestInfo map[string]SearchRequestInformation
	mux sync.Mutex
}

type SearchRequestInformation struct {
	Keywords []string
	NbOfMatches uint32
	BudgetIsGiven bool
}

type FileAndChunkInformation struct {
	// do we need an array of filenames ? otherwise we keep the filename of the first response we get
	Filename string
	Metahash string
	Metafile string
	NbOfChunks uint64
	FoundAllChunks bool
	AlreadyShown bool
	ChunkOriginToIndices map[string][]uint64
}

type SafeOriginAndKeyword struct {
	OriginAndKeywords map[OriginAndKeywordsStruct]bool
	mux sync.Mutex
}

type OriginAndKeywordsStruct struct {
	Origin string
	Keywords string
}

type FileChunkInfoAndNbMatches struct {
	FilesAndChunksInfo []FileAndChunkInformation
	NbOfMatches uint32
}

type SafeKeywordToInformation struct {
	KeywordToInfo map[string]FileChunkInfoAndNbMatches
	mux sync.Mutex
}