package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/loisBN/zippytal-desktop/back/manager"
	"google.golang.org/grpc"
)


func main() {
	lis,err := net.Listen("tcp",":8080")
	if err != nil {
		log.Fatalln(err)
	}
	m,err := manager.NewManager()
	if err != nil {
		log.Fatal(err)
	}
	h := manager.NewWSHandler(m,[]manager.WSMiddleware{manager.NewWSStateMiddleware()},[]manager.HTTPMiddleware{&manager.SquadHTTPMiddleware{}})
		serv := manager.NewWSServ(":9999",h)
		fmt.Println("server launch")
		certFile := "/etc/letsencrypt/live/app.zippytal.com/fullchain.pem"
		keyFile := "/etc/letsencrypt/live/app.zippytal.com/privkey.pem"
		certFileW := "/etc/letsencrypt/live/zippytal.com/fullchain.pem"
		keyFileW := "/etc/letsencrypt/live/zippytal.com/privkey.pem"
	go func() {
		log.Fatalln(serv.Server.ListenAndServeTLS(certFile,keyFile))
	}()
	go func() {
		tlsConfig := &tls.Config{}
		tlsConfig.Certificates = make([]tls.Certificate, 2)
		var err error
		tlsConfig.Certificates[0], err = tls.LoadX509KeyPair(certFile,keyFile)
		if err != nil {
			log.Fatalln(err)
		}
		tlsConfig.Certificates[1], err = tls.LoadX509KeyPair(certFileW,keyFileW)
		if err != nil {
			log.Fatalln(err)
		}
		http.Handle("app.zippytal.com/",h)
		http.Handle("https://app.zippytal.com/",h)
		http.HandleFunc("zippytal.com/",func(rw http.ResponseWriter, r *http.Request) {
			if _,err :=  os.Stat("./website/"+r.URL.Path); os.IsNotExist(err) {
				http.ServeFile(rw,r,"./website/index.html")
			} else {
				http.ServeFile(rw,r,"./website/" + r.URL.Path)
			}
		})
		s := &http.Server{
			TLSConfig: tlsConfig,
		}
		lis,err := tls.Listen("tcp",":443",tlsConfig)
		if err != nil {
			log.Fatalln(err)
		}
		log.Fatalln(s.Serve(lis))
	}()
	grpcServer := grpc.NewServer(grpc.MaxConcurrentStreams(100000))
	manager.RegisterGrpcManagerServer(grpcServer,manager.NewGRPCManagerService(m))
	log.Fatalln(grpcServer.Serve(lis))
}