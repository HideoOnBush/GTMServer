package quicClient

import (
	"context"
	"crypto/tls"
	"github.com/quic-go/quic-go"
	"io"
	"log"
)

const serviceName = "GetRelation"

type QClient struct {
}

func Initialize(address string) *QClient {
	return &QClient{}
}

//func (q *QClient) CheckService(serviceName string) (serviceAddress []string, err error) {
//	key := fmt.Sprintf("/services/%s", serviceName)
//	resp, err := q.Get(context.Background(), key, clientv3.WithPrefix())
//	if err != nil {
//		err = fmt.Errorf("get failed in Etcd,err=%v", err)
//		return
//	}
//	for _, ev := range resp.Kvs {
//		key := string(ev.Key)
//		lastSlashIndex := strings.LastIndex(key, "/")
//
//		// 取出最后一个 / 后面的内容
//		result := key[lastSlashIndex+1:]
//		serviceAddress = append(serviceAddress, result)
//	}
//	return
//}

func (q *QClient) BLServiceChoice(services []string) string {
	//TODO
	return services[0]
}

func (q *QClient) DialQuic(ctx context.Context, data []byte) (string, error) {
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

	//discover the services
	//address, err := q.CheckService(serviceName)
	//if err != nil {
	//	log.Printf("CheckService err = %v", err)
	//	return err
	//}
	//choose one service through BalanceLoad
	//targetAddr := q.BLServiceChoice(address)

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic"},
	}
	conn, err := quic.DialAddr(context.Background(), "127.0.0.1:2234", tlsConf, nil)
	if err != nil {
		log.Printf("InitializeClient Dial err = %v", err)
		return "", err
	}
	defer conn.CloseWithError(0, "")
	//ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//defer cancel()
	str, err := conn.OpenStreamSync(ctx)
	if err != nil {
		log.Printf("InitializeClient OpenUniStreamSync err = %v", err)
		return "", err
	} else {
		//log.Printf("have strId = %v", str.StreamID())
	}
	defer str.Close()

	//line, _ := json.Marshal(Line{
	//	Source:       "ss",
	//	SourceIsCore: false,
	//	SourceScene:  "test",
	//	Target:       "tt",
	//	TargetIsCore: false,
	//	TargetScene:  "test",
	//	Dependence:   "weak",
	//})
	_, err = str.Write(data)
	if err != nil {
		log.Printf("InitializeClient Write err = %v", err)
		return "", err
	}
	buf := make([]byte, len(data))
	_, err = io.ReadFull(str, buf)
	if err != nil {
		log.Printf("InitializeClient ReadFull err = %v", err)
		return "", err
	}
	//fmt.Printf("Client: Got '%s'\n", buf)
	return string(buf), nil
}
