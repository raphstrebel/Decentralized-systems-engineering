
const SERVER_ADDRESS = "http://localhost:8080";

var messages = [];
var last_msg_show_id = -1;
var peers = [];
var last_peer_show_id = -1;
var close_nodes = [];
var last_close_node_id = -1;
var privateNode;

// Fetch the Peer ID
function getMyID() {
	(function() {$.getJSON(
	 	SERVER_ADDRESS + "/id",
		function(json) {
		  	const paragraph = document.createElement('h2');
			paragraph.textContent = json.ID;
			container_element = document.getElementById('idBoxID');
			container_element.appendChild(paragraph);
		}
	)
	;}) ()
}

// Periodically send GET to server and update message list if necessary
function getNewMessages() {
	(function() {$.getJSON(
	 	SERVER_ADDRESS + "/message",
		function(json) {
			msg_string = JSON.stringify(json);
			msgs = JSON.parse(msg_string);

			msgs.Message.map(x => messages.push(x));
			showMessages();
	}
	)
	;}) ()
}

// Periodically send GET to server and update node list if necessary
function getNewNodes() {
	(function() {$.getJSON(
	 	SERVER_ADDRESS + "/node",
		function(json) {
			peers_str = JSON.stringify(json);
			peers_parsed = JSON.parse(peers_str);			
			peers_parsed.Node.map(x => peers.push(x));
			showAllNodes();
		}
	)
	;}) ()
}

// Periodically send GET to server and update "close nodes" list if necessary
function getNewCloseNodes() {
	(function() {$.getJSON(
	 	SERVER_ADDRESS + "/route",
		function(json) {
			close_nodes_str = JSON.stringify(json);
			close_nodes_parsed = JSON.parse(close_nodes_str);			
			close_nodes_parsed.CloseNode.map(x => close_nodes.push(x));
			showAllCloseNodes();
		}
	)
	;}) ()
}

function sendMessage() {
	// First add the message to our list of messages
	const messageInput = document.getElementById('newMessageID');

	if(messageInput.value == "") {
		return;
	}

	// Then send message to server :	
	$.ajax({
	  type: "POST",
	  url: SERVER_ADDRESS + '/message', 
	  data: {Message: messageInput.value, Update: true},
	  dataType: 'jsonp'
	});
}	

function sendNode() {
	// First add the node to our list of nodes
	const nodeInput = document.getElementById('newNodeID');

	if(nodeInput.value == "") {
		return;
	}

	// Then send node to server :
	$.ajax({
	  type: "POST",
	  url: SERVER_ADDRESS + '/node',
	  data: {Node: nodeInput.value, Update: true},
	  dataType: 'jsonp'
	});
}

/*function sendCloseNode() {
	// First add the node to our list of nodes
	const closeNode = document.getElementById('closeNodeID');
	const newMessage = document.getElementById('newPrivateMessageID');
	//addNode(nodeInput.value);

	if(closeNode.value == "" || newMessage.value == "") {
		return;
	}

	// Then send node to server :
	$.ajax({
	  type: "POST",
	  url: SERVER_ADDRESS + '/route',
	  data: {CloseNode: closeNode.value, Message: newMessage.value, Update: true},
	  dataType: 'jsonp'
	});
}*/

function sendPrivateMessage() {
	// First add the message to our list of messages
	const messageInput = document.getElementById('newPrivateMessageID');

	if(messageInput.value == "") {
		return;
	}

	// Then send message to server :	
	$.ajax({
	  type: "POST",
	  url: SERVER_ADDRESS + '/message', 
	  data: {Destination: privateNode, PrivateMessage: messageInput.value, Update: true},
	  dataType: 'jsonp'
	});
}	

// Prints the list of messages in initial array
function showMessages() {
	// Create a paragraph (line) for each message
	messages.forEach((m, index) => {
		if(index > last_msg_show_id) {
			addMessage(m);
			last_msg_show_id++;
		}
	});
}

// Prints the list of peers in initial array
function showAllNodes() {
	// Create a paragraph (line) for each peer
	peers.forEach((p, index) => {
		if(index > last_peer_show_id) {
			addNode(p);
			last_peer_show_id++;
		}
	});
}

// Prints the list of close nodes in initial array
function showAllCloseNodes() {
	// Create a paragraph (line) for each peer
	close_nodes.forEach((c, index) => {
		if(index > last_close_node_id) {
			addCloseNode(c);
			last_close_node_id++;
		}
	});
}

// Adds message "message" to the list of messages in the chatbox
function addMessage(message) {
	const paragraph = document.createElement('li');
	paragraph.textContent = message;
	container_element = document.getElementById('chatboxID');
	container_element.appendChild(paragraph);
}

// Adds a new peer to the list of peers in the nodeBox
function addNode(node) {
	const paragraph = document.createElement('p');
	paragraph.textContent = node;
	container_element = document.getElementById('nodeBoxID');
	container_element.appendChild(paragraph);
}

// Adds a new peer to the list of peers in the nodeBox
function addCloseNode(node) {
	var button = document.createElement('addCloseNodeButton');
	const privateChat = document.getElementById('privateChatboxID');
	button.textContent = node;

	button.onclick = function(){
		privateNode = node;
		privateChat.style.display = "block";
		/*const paragraph = document.createElement('h2');
		paragraph.textContent = node;
		container_element = document.getElementById('privateChatboxID');
		container_element.appendChild(paragraph);*/
	}

	window.onclick = function(event) {
	    if (event.target == privateChat) {
	        privateChat.style.display = "none";
	    }
	}

	container_element = document.getElementById('closeNodesBoxID');
	container_element.appendChild(button);
}

function shareFile() {
	var file = document.getElementById("fileID");
	console.log("file opened");
}

window.onload = function() {
	showMessages();
	showAllNodes();
	getMyID();

	/* Periodic 1sec get
    setInterval(function(){ 
		getNewMessages();
		getNewNodes(); 
		getNewCloseNodes();
	}, 3000);*/
};


