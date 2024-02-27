package quicClient

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/quic-go/quic-go"
	"io"
	"log"
)

type QClient struct {
	TargetAddr string
}

func Initialize(address string) *QClient {
	return &QClient{
		TargetAddr: address,
	}
}

func (q *QClient) DialQuic(ctx context.Context) error {
	//ctx, cancel := context.WithTimeout(ctx, 3*time.Second) // 3s handshake timeout
	//defer cancel()
	//targetAddr, err := net.ResolveUDPAddr("udp4", q.TargetAddr)
	//if err != nil {
	//	log.Printf("InitializeClient ResolveUDPAddr err = %v", err)
	//	return err
	//}
	//udpConn, err := net.DialUDP("udp4", nil, targetAddr)
	//if err != nil {
	//	log.Printf("InitializeClient DialUDPDialUDP err = %v", err)
	//	return err
	//}
	//tr := quic.Transport{
	//	Conn: udpConn,
	//}
	//
	//conn, err := tr.DialEarly(ctx, targetAddr, &tls.Config{}, nil)
	//conn, err := tr.Dial(ctx, targetAddr, &tls.Config{}, nil)
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic"},
	}
	conn, err := quic.DialAddr(context.Background(), q.TargetAddr, tlsConf, nil)
	if err != nil {
		log.Printf("InitializeClient Dial err = %v", err)
		return err
	}
	defer conn.CloseWithError(0, "")
	//ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer cancel()
	str, err := conn.OpenStreamSync(ctx)
	if err != nil {
		log.Printf("InitializeClient OpenUniStreamSync err = %v", err)
		return err
	} else {
		log.Printf("have strId = %v", str.StreamID())
	}
	defer str.Close()
	line, _ := json.Marshal(Line{
		Source:       "ss",
		SourceIsCore: false,
		SourceScene:  "test",
		Target:       "tt",
		TargetIsCore: false,
		TargetScene:  "test",
		Dependence:   "weak",
	})
	_, err = str.Write(line)
	if err != nil {
		log.Printf("InitializeClient Write err = %v", err)
		return err
	}

	buf := make([]byte, len(line))
	_, err = io.ReadFull(str, buf)
	if err != nil {
		log.Printf("InitializeClient ReadFull err = %v", err)
		return err
	}
	fmt.Printf("Client: Got '%s'\n", buf)
	return nil
}
