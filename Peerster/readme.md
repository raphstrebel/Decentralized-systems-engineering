Homework 3 :

Note On file naming for ex 1:
When searching for a file and downloading it, I name the file with the same name as any other peer has given it (we do not really care about that) if this name is available.
However, in the case where the name is already given to another file, I name it with a random appended alphanumerical character at the beginning. For example if "carlton.txt"
is already used in my "SharedFiles" folder, then it might be named "Kcarlton.txt", or "2Kcarlton.txt" if "Kcarlton.txt" is also already used (and so on).








Homework 2 :

By default the frontend will be available, as is the antiEntropy process. To run the tests change the line 553 in "Peerster/main.go" and simply change "go antiEntropy(gossiper)" to "antiEntropy(gossiper)".

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