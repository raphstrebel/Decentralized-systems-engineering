README :

The code I hand in can be tested for Rumor Messages without changes if one does not use the frontend (GUI disabled by default). 
However to be able to test the Simple message functionality you should open the /client/main.go file and comment lines 48 to 58 and un-comment lines 42 and 43.

To be able to test the frontend you must do the following.

First of all, the imports :
You might need to change some imports from "mux" to "github/gorilla/mux". 
I added the following github libraries :
"protobuf"
"mux"
"handlers"
"simplejson"

- In the Peerser/main.go file :

1) Un-comment the following imports (so all imports should be accessible) :
"mux"
"handlers"
"simplejson"
"net/http"
"regexp"

2) Un-comment entirely the handler functions, so :
- IDHandler		     : erase the "backslash-star" characters at lines 642 and 655
- MessageHandler	 : erase the "backslash-star" characters at lines 667 and 727
- NodeHandler		 : erase the "backslash-star" characters at lines 740 and 782

3) Inside the main() :
- At line 803 : add a "go " so that you have "go antiEntropy(gossiper)" instead of simply "antiEntropy(gossiper)"
- erase the "backslash-star" characters at lines 805 and 819.



HOMEWORK 2 :

I assume that rtimer is greater or equal to 2 (otherwise I must change the GET methods in the frontend to get new messages less than every second, which does not really make sense).