
const SERVER_ADDRESS = "http://localhost:8080";

var messages = [];
var last_msg_show_id = -1;
var peers = [];
var last_peer_show_id = -1;
var close_nodes = [];
var last_close_node_id = -1;
var privateNode;
var myID = -1;
var private_messages_origins = [];
var private_messages_texts = [];
var last_priv_msg_shown_id = -1;

// Fetch the Peer ID
function getMyID() {
	(function() {$.getJSON(
	 	SERVER_ADDRESS + "/id",
		function(json) {
			myID = json.ID;
		  	const paragraph = document.createElement('h2');
			paragraph.textContent = myID;
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

// Periodically send GET to server and update private message list if necessary
// Idea : append the nodeID to message so that we split and keep only nodeID and 
function getNewPrivateMessages() {
	(function() {$.getJSON(
	 	SERVER_ADDRESS + "/privateMessage",
		function(json) {
			priv_msg_string = JSON.stringify(json);
			priv_msgs = JSON.parse(priv_msg_string);

			priv_msgs.PrivateMessageOrigin.map(x => private_messages_origins.push(x));
			priv_msgs.PrivateMessageText.map(x => private_messages_texts.push(x));

			showPrivateMessages();
	}
	)
	;}) ()
}

function sendMessage() {
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

	messages.push(messageInput.value)
	showMessages()

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

function sendPrivateMessage() {
	const messageInput = document.getElementById('newPrivateMessageID');

	if(messageInput.value == "") {
		return;
	}

	// Then send message to server :	
	$.ajax({
	  type: "POST",
	  url: SERVER_ADDRESS + '/privateMessage', 
	  data: {Destination: privateNode, PrivateMessage: messageInput.value, Update: true},
	  dataType: 'jsonp'
	});

	private_messages_texts.push(messageInput.value)
	private_messages_origins.push(myID)
	showPrivateMessages()
}	

// Get a file of a close node by entering hexa metahash
function sendNodeFileRequest() {
	const metahash = document.getElementById("hexaMetahashID");
	const fileName = document.getElementById("fileNameID");

	if(metahash.value != "" && fileName.value != "") {
		// Send filepath to server :	
		$.ajax({
		  type: "POST",
		  url: SERVER_ADDRESS + '/fileSharing', 
		  data: {FileName: fileName.value, Destination: privateNode, Metahash: metahash.value},
		  dataType: 'jsonp'
		});

	}	
}

function sendFileRequest() {
	const keywords = document.getElementById("keywordsID").value;
	const budget = document.getElementById("budgetID").value;
	var budgetToSend;
	
	if(budget != "" && budget >= 0) {
		budgetToSend = budget;
	} 

	if(keywords != "") {
		$.ajax({
		  type: "POST",
		  url: SERVER_ADDRESS + '/fileSearching', 
		  data: {Keywords: keywords, Budget: budgetToSend},
		  dataType: 'jsonp'
		});
	}
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

// Prints the list of messages in initial array
function showPrivateMessages() {
	// Create a paragraph (line) for each message
	private_messages_texts.forEach((m, index) => {
		if(index > last_priv_msg_shown_id) {
			addPrivateMessage(m, private_messages_origins[index]);
			last_priv_msg_shown_id++;
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
	}

	window.onclick = function(event) {
	    if (event.target == privateChat) {
	        privateChat.style.display = "none";
	    }
	}

	container_element = document.getElementById('closeNodesBoxID');
	container_element.appendChild(button);
}

function addPrivateMessage(message, origin) {
	const paragraph = document.createElement('li');
	if(origin !== myID) {
		paragraph.textContent = "Private message from : " + origin + " : \n" + message;
	} else {
		paragraph.textContent = "Private message to " + privateNode + " : \n" + message;
	}

	container_element = document.getElementById('chatboxID');
	container_element.appendChild(paragraph);
}

function shareFile() {
	var filename = getFileName(document.getElementById("fileID"));

	if(filename == "") {
		return;
	}

	// Then send filepath to server :	
	$.ajax({
	  type: "POST",
	  url: SERVER_ADDRESS + '/fileSharing', 
	  data: {FileName: filename, Update: true},
	  dataType: 'jsonp'
	});
}

// help from : https://stackoverflow.com/questions/857618/javascript-how-to-extract-filename-from-a-file-input-control
function getFileName(file) {
	//var file = document.getElementById("fileID");
	var fullPath = file.value;

	if (fullPath) {
	    var startIndex = (fullPath.indexOf('\\') >= 0 ? fullPath.lastIndexOf('\\') : fullPath.lastIndexOf('/'));
	    var filename = fullPath.substring(startIndex);
	    if (filename.indexOf('\\') === 0 || filename.indexOf('/') === 0) {
	        filename = filename.substring(1);
	    }
	}

	return filename;
}

window.onload = function() {
	getMyID();
	showMessages();
	showAllNodes();

	// Periodic 1sec get
    setInterval(function(){ 
		getNewMessages();
		getNewNodes(); 
		getNewCloseNodes();
		getNewPrivateMessages();
	}, 3000);
};


