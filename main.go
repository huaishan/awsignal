package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/huaishan/awsignal/signalsrv"
)

var addr = flag.String("addr", "0.0.0.0:8000", "http service address")

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1048576,
	WriteBufferSize: 1048576,
}

func main() {
	flag.Parse()
	apps := []*signalsrv.AppConfig{
		{Path: "/", AppName: "Test", AddressSharing: false},
		{Path: "/chatapp", AppName: "ChatApp", AddressSharing: false},
		{Path: "/callapp", AppName: "CallApp", AddressSharing: false},
		{Path: "/conferenceapp", AppName: "ConferenceApp", AddressSharing: true},
		{Path: "/test", AppName: "UnitTests", AddressSharing: false},
		{Path: "/testshared", AppName: "UnitTestsAddressSharing", AddressSharing: true},
	}

	srv := &http.Server{
		Addr:         *addr,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	wns := signalsrv.NewWebsocketNetworkServer()
	for _, conf := range apps {
		http.HandleFunc(conf.Path, func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Println(err)
				return
			}
			wns.OnConnection(conn, conf)
		})
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err.Error())
		}
	}()
	log.Println("websockets/http listening on ", *addr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutdown Server...")

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal(err.Error())
	}
}
