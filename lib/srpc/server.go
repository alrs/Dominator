package srpc

import (
	"errors"
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"
)

const (
	connectString = "200 Connected to Go SRPC"
	rpcPath       = "/_goSRPC_/"
)

type receiverType struct {
	methods map[string]reflect.Value
}

var receivers map[string]receiverType = make(map[string]receiverType)

// Precompute the reflect type for net.Conn. Can't use net.Conn directly
// because Typeof takes an empty interface value. This is annoying.
var typeOfConn = reflect.TypeOf((**Conn)(nil)).Elem()

func init() {
	http.HandleFunc(rpcPath, httpHandler)
}

func registerName(name string, rcvr interface{}) error {
	var receiver receiverType
	receiver.methods = make(map[string]reflect.Value)
	typeOfReceiver := reflect.TypeOf(rcvr)
	valueOfReceiver := reflect.ValueOf(rcvr)
	for index := 0; index < typeOfReceiver.NumMethod(); index++ {
		method := typeOfReceiver.Method(index)
		if method.PkgPath != "" { // Method must be exported.
			continue
		}
		methodType := method.Type
		// Method needs two ins: receiver, *Conn.
		if methodType.NumIn() != 2 {
			continue
		}
		if methodType.In(1) != typeOfConn {
			continue
		}
		if methodType.NumOut() != 0 {
			continue
		}
		receiver.methods[method.Name] = valueOfReceiver.Method(index)
	}
	receivers[name] = receiver
	return nil
}

func httpHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	conn, bufrw, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Println("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	defer conn.Close()
	io.WriteString(conn, "HTTP/1.0 "+connectString+"\n\n")
	myConn := new(Conn)
	myConn.ReadWriter = bufrw
	handleConnection(myConn)
}

func handleConnection(conn *Conn) {
	defer conn.Flush()
	for ; ; conn.Flush() {
		serviceMethod, err := conn.ReadString('\n')
		if err == io.EOF {
			return
		}
		if err != nil {
			log.Println(err)
			conn.WriteString(err.Error() + "\n")
			continue
		}
		serviceMethod = serviceMethod[:len(serviceMethod)-1]
		method, err := findMethod(serviceMethod)
		if err != nil {
			conn.WriteString(err.Error() + "\n")
			continue
		} else {
			conn.WriteString("\n")
		}
		conn.Flush()
		method.Call([]reflect.Value{reflect.ValueOf(conn)})
	}
}

func findMethod(serviceMethod string) (*reflect.Value, error) {
	splitServiceMethod := strings.Split(serviceMethod, ".")
	if len(splitServiceMethod) != 2 {
		return nil, errors.New("malformed Service.Method: " + serviceMethod)
	}
	serviceName := splitServiceMethod[0]
	receiver, ok := receivers[serviceName]
	if !ok {
		return nil, errors.New("unknown service: " + serviceName)
	}
	methodName := splitServiceMethod[1]
	method, ok := receiver.methods[methodName]
	if !ok {
		return nil, errors.New(serviceName + ": unknown method: " + methodName)
	}
	return &method, nil
}