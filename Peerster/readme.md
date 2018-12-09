Homework 3 :

Notes on file naming for ex 1:
- In the GUI, when searching for a file and downloading it, as we do not give a default name, I name the file with the same name as any other peer we are downloading from has given it (we do not really care about that) if this name is available.
However, in the case where the name is already given to another file, I name it with a random appended alphanumerical character at the beginning. For example if "carlton.txt"
is already used in my "SharedFiles" folder, then it might be named "Kcarlton.txt", or "2Kcarlton.txt" if "Kcarlton.txt" is also already used (and so on).
- When doing a search with specified budget, I check if the number of matches is (at least) 2 for every second during the first 5 seconds. Then I stop checking for matches.



Notes on blockchain of ex 2:
- As asked in the assignment, I do not accept a block when I don't have the prevHash. However, I do accept a block when its prevHash is "0000...".
- I assume a block with no transactions is valid
- I print and send all FOUND-BLOCK's, even when mining an empty block

Also, the import libraries are named under "github.com/dedis/protobuf" instead of "protobuf" as I didn't know which you need. If it does not run please change the necessary libraries.

All imports used :

"fmt"
"flag"
"github.com/gorilla/mux"
"github.com/dedis/protobuf"
"github.com/bitly/go-simplejson"
"math/rand"
"handlers"
"bytes"
"net/http"
"regexp"
"sync"
"bufio"
"crypto/sha256"
"hash"
"os"
"io"
"reflect"
"time"
"net"
"strings"
"encoding/hex"

(On my OS I added the libraries "mux", "protobuf" and "simple-json" directy to the GOPATH so I hope I have not forgotten to change one import from "protobuf" to "github.com/dedis/protobuf" for example. If the main does not build please change the imports, I was careful but a mistake can happen easily).