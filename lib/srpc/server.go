package srpc

import (
	"bufio"
	"crypto/tls"
	"errors"
	"github.com/Symantec/Dominator/lib/x509util"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

const (
	connectString   = "200 Connected to Go SRPC"
	rpcPath         = "/_goSRPC_/"
	tlsRpcPath      = "/_go_TLS_SRPC_/"
	listMethodsPath = rpcPath + "listMethods"
)

type receiverType struct {
	methods map[string]reflect.Value
}

var receivers map[string]receiverType = make(map[string]receiverType)

// Precompute some reflect types. Can't use the types directly because Typeof
// takes an empty interface value. This is annoying.
var typeOfConn = reflect.TypeOf((**Conn)(nil)).Elem()
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

func init() {
	http.HandleFunc(rpcPath, unsecuredHttpHandler)
	http.HandleFunc(tlsRpcPath, tlsHttpHandler)
	http.HandleFunc(listMethodsPath, listMethodsHttpHandler)
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
		if methodType.NumOut() != 1 {
			continue
		}
		if methodType.Out(0) != typeOfError {
			continue
		}
		receiver.methods[method.Name] = valueOfReceiver.Method(index)
	}
	receivers[name] = receiver
	return nil
}

func unsecuredHttpHandler(w http.ResponseWriter, req *http.Request) {
	httpHandler(w, req, false)
}

func tlsHttpHandler(w http.ResponseWriter, req *http.Request) {
	httpHandler(w, req, true)
}

func httpHandler(w http.ResponseWriter, req *http.Request, doTls bool) {
	if doTls && serverTlsConfig == nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if (tlsRequired && !doTls) || req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	unsecuredConn, bufrw, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Println("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	if tcpConn, ok := unsecuredConn.(*net.TCPConn); ok {
		if err := tcpConn.SetKeepAlive(true); err != nil {
			log.Println("error setting keepalive: ", err.Error())
			return
		}
		if err := tcpConn.SetKeepAlivePeriod(time.Minute * 5); err != nil {
			log.Println("error setting keepalive period: ", err.Error())
			return
		}
	}
	_, err = io.WriteString(unsecuredConn, "HTTP/1.0 "+connectString+"\n\n")
	if err != nil {
		log.Println("error writing connect message: ", err.Error())
		return
	}
	myConn := new(Conn)
	if doTls {
		tlsConn := tls.Server(unsecuredConn, serverTlsConfig)
		defer tlsConn.Close()
		if err := tlsConn.Handshake(); err != nil {
			log.Println(err)
			return
		}
		myConn.username, myConn.permittedMethods, err = getAuth(
			tlsConn.ConnectionState())
		if err != nil {
			log.Println(err)
			return
		}
		myConn.ReadWriter = bufio.NewReadWriter(bufio.NewReader(tlsConn),
			bufio.NewWriter(tlsConn))
	} else {
		defer unsecuredConn.Close()
		myConn.ReadWriter = bufrw
	}
	handleConnection(myConn)
}

func getAuth(state tls.ConnectionState) (string, map[string]struct{}, error) {
	var username string
	permittedMethods := make(map[string]struct{})
	for _, certChain := range state.VerifiedChains {
		for _, cert := range certChain {
			var err error
			if username == "" {
				username, err = x509util.GetUsername(cert)
				if err != nil {
					return "", nil, err
				}
			}
			pms, err := x509util.GetPermittedMethods(cert)
			if err != nil {
				return "", nil, err
			}
			for method := range pms {
				permittedMethods[method] = struct{}{}
			}
		}
	}
	return username, permittedMethods, nil
}

func handleConnection(conn *Conn) {
	defer conn.Flush()
	for ; ; conn.Flush() {
		serviceMethod, err := conn.ReadString('\n')
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		if err != nil {
			log.Println(err)
			if _, err := conn.WriteString(err.Error() + "\n"); err != nil {
				log.Println(err)
				return
			}
			continue
		}
		if serviceMethod == "\n" {
			// Received a "ping" request, send response.
			if _, err := conn.WriteString("\n"); err != nil {
				log.Println(err)
				return
			}
			continue
		}
		serviceMethod = serviceMethod[:len(serviceMethod)-1]
		method, err := findMethod(serviceMethod)
		if err != nil {
			if _, err := conn.WriteString(err.Error() + "\n"); err != nil {
				log.Println(err)
				return
			}
			continue
		}
		if !conn.checkPermitted(serviceMethod) {
			if _, e := conn.WriteString(
				ErrorAccessToMethodDenied.Error() + "\n"); e != nil {
				log.Println(e)
				return
			}
			continue
		} else {
			if _, err := conn.WriteString("\n"); err != nil {
				log.Println(err)
				return
			}
		}
		if err := conn.Flush(); err != nil {
			log.Println(err)
			return
		}
		returnValues := method.Call([]reflect.Value{reflect.ValueOf(conn)})
		errInter := returnValues[0].Interface()
		if errInter != nil {
			err = errInter.(error)
			log.Println(err)
			return
		}
	}
}

// Returns true if the method is permitted, else false if denied.
func (conn *Conn) checkPermitted(serviceMethod string) bool {
	if conn.permittedMethods == nil {
		return true
	}
	for sm := range conn.permittedMethods {
		if matched, _ := filepath.Match(sm, serviceMethod); matched {
			return true
		}
	}
	return false
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

func listMethodsHttpHandler(w http.ResponseWriter, req *http.Request) {
	writer := bufio.NewWriter(w)
	defer writer.Flush()
	methods := make([]string, len(receivers))
	for receiverName, receiver := range receivers {
		for method := range receiver.methods {
			methods = append(methods, receiverName+"."+method+"\n")
		}
	}
	sort.Strings(methods)
	for _, method := range methods {
		writer.WriteString(method)
	}
}
