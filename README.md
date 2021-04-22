# Decentralized-systems-engineering

Implementation of a peer-to-peer file exchange and chat application.

Upon cloning the repository, run `go run ./Peerster -UIPort=<UI_port> -gossipAddr=127.0.0.1:<own_port> -peers=127.0.0.1:<peer_1_port> -name=<name> -rtimer=3` to launch the first node. 
An optional list of peers can be added by comma-separated `127.0.0.1:<peer_port>` values. The peers have to be activated by running the same `go run ...` command with their own ports.

The (very basic) frontend is located in Peerster/GUI:
- add other peers
- upload files
- send file to neighbour peer
- access files uploaded by peers for which there exists a sequence of neighbours
